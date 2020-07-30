package main

import (
	"flag";
	"context";
	"runtime";
	"sync";
	"time";
	"encoding/binary";
	"strings";

	"github.com/marekgalovic/anndb/index";
	// "github.com/marekgalovic/anndb/tau";
	"github.com/marekgalovic/anndb/index/space";

	"github.com/satori/go.uuid";
	"gonum.org/v1/hdf5";
	log "github.com/sirupsen/logrus";
)

const topK int = 10

func getDataset(f *hdf5.File, name string) (*hdf5.Dataset, []uint, error) {
	d, err := f.OpenDataset(name)
	if err != nil {
		return nil, nil, err
	}

	s := d.Space()
	defer s.Close()

	dims, _, err := s.SimpleExtentDims()
	if err != nil {
		d.Close()
		return nil, nil, err
	}

	return d, dims, nil
}

func readDim100(dataset *hdf5.Dataset, len uint) ([][]float32, error) {
	v := make([][100]float32, len)
	if err := dataset.Read(&v); err != nil {
		return nil, err
	}
	
	result := make([][]float32, len)
	for i, vec := range v {
		result[i] = make([]float32, 100)
		copy(result[i], vec[:])
	}
	return result, nil
}

func readDim128(dataset *hdf5.Dataset, len uint) ([][]float32, error) {
	v := make([][128]float32, len)
	if err := dataset.Read(&v); err != nil {
		return nil, err
	}
	
	result := make([][]float32, len)
	for i, vec := range v {
		result[i] = make([]float32, 128)
		copy(result[i], vec[:])
	}
	return result, nil
}

func readDim784(dataset *hdf5.Dataset, len uint) ([][]float32, error) {
	v := make([][784]float32, len)
	if err := dataset.Read(&v); err != nil {
		return nil, err
	}

	result := make([][]float32, len)
	for i, vec := range v {
		result[i] = make([]float32, 784)
		copy(result[i], vec[:])
	}
	return result, nil
}

func readNeighbors(dataset *hdf5.Dataset, len uint) ([][]uuid.UUID, error) {
	v := make([][100]int32, len)
	if err := dataset.Read(&v); err != nil {
		return nil, err
	}

	result := make([][]uuid.UUID, len)
	for i, vec := range v {
		result[i] = make([]uuid.UUID, 100)
		for j, id := range vec {
			_uuid := make([]byte, 16)
			binary.LittleEndian.PutUint64(_uuid[:8], uint64(id))
			result[i][j] = uuid.Must(uuid.FromBytes(_uuid))
		}
	}
	return result, nil
}

type insertItem struct {
	id uuid.UUID
	value []float32
}

func insertWorker(ctx context.Context, index *index.Hnsw, tasks <- chan *insertItem, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case item := <- tasks:
			if item == nil {
				return
			}
			if err := index.Insert(item.id, item.value, nil, index.RandomLevel()); err != nil {
				log.Error(err)
			}
		case <- ctx.Done():
			return
		}
	}
}

type searchTask struct {
	query []float32
	neighborIds []uuid.UUID
}

func searchWorker(ctx context.Context, index *index.Hnsw, tasks <- chan *searchTask, wg *sync.WaitGroup, rp *float64) {
	defer wg.Done()

	var totalRecall float64 = 0
	for {
		select {
		case task := <- tasks:
			if task == nil {
				*rp = totalRecall
				return
			}
			result, _ := index.Search(ctx, task.query, uint(topK))
			totalRecall += recall(result, task.neighborIds, topK)
		case <- ctx.Done():
			return
		}
	}
}

func recall(result index.SearchResult, neighbors []uuid.UUID, k int) float64 {
	topKNeighbors := make(map[uuid.UUID]struct{})
	for i := 0; i < k; i++ {
		topKNeighbors[neighbors[i]] = struct{}{}
	}

	hits := 0
	for _, ri := range result {
		if _, exists := topKNeighbors[ri.Id]; exists {
			hits++
		}
	}

	return float64(hits) / float64(k)
}

func main() {
	var filePath string
	var hnswM int
	flag.StringVar(&filePath, "file", "", "Dataset file")
	flag.IntVar(&hnswM, "hnsw-m", 4, "M")
	flag.Parse()

	f, err := hdf5.OpenFile(filePath, hdf5.F_ACC_RDONLY)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	train, dims, err := getDataset(f, "train")
	if err != nil {
		log.Fatal(err)
	}
	defer train.Close()

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	spaceParts := strings.Split(strings.TrimRight(filePath, ".hdf5"), "-")
	spaceName := spaceParts[len(spaceParts)-1]
	log.Infof("Space: %s", spaceName)

	var _space space.Space
	switch spaceName {
	case "euclidean":
		_space = space.NewEuclidean()
	case "manhattan":
		_space = space.NewManhattan()
	case "angular":
		_space = space.NewCosine()
	default:
		log.Fatal("Invalid space name")
	}

	idx := index.NewHnsw(dims[1], _space, index.HnswEfConstruction(500), index.HnswM(hnswM))

	// Insert data to index
	wg := &sync.WaitGroup{}
	tasks := make(chan *insertItem)

	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go insertWorker(ctx, idx, tasks, wg)
	}

	var trainData [][]float32
	switch dims[1] {
	case 100:
		trainData, err = readDim100(train, dims[0])
	case 128:
		trainData, err = readDim128(train, dims[0])
	case 784:
		trainData, err = readDim784(train, dims[0])
	default:
		log.Fatal("Invalid dimension")
	}
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Build index ...")
	startAt := time.Now()
	for id, value := range trainData {
		_uuid := make([]byte, 16)
		binary.LittleEndian.PutUint64(_uuid[:8], uint64(id))
		tasks <- &insertItem {uuid.Must(uuid.FromBytes(_uuid)), value[:]}
	}

	close(tasks)
	wg.Wait()
	d := time.Since(startAt)

	if len(trainData) != idx.Len() {
		log.Fatal("Incomplete index")
	}
	log.Infof("Built in: %s (%.2f inserts/s)", d, float64(dims[0]) / d.Seconds())

	// Get test data
	test, dims, err := getDataset(f, "test")
	if err != nil {
		log.Fatal(err)
	}
	defer train.Close()

	var testData [][]float32
	switch dims[1] {
	case 100:
		testData, err = readDim100(test, dims[0])
	case 128:
		testData, err = readDim128(test, dims[0])
	case 784:
		testData, err = readDim784(test, dims[0])
	default:
		log.Fatal("Invalid dimension")
	}
	if err != nil {
		log.Fatal(err)
	}

	// Get ground truth
	neighbors, dims, err := getDataset(f, "neighbors")
	if err != nil {
		log.Fatal(err)
	}
	defer neighbors.Close()

	neighborIds, err := readNeighbors(neighbors, dims[0])
	if err != nil {
		log.Fatal(err)
	}

	// Start search workers
	wg = &sync.WaitGroup{}
	searchTasks := make(chan *searchTask)

	recallAcc := make([]float64, runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go searchWorker(ctx, idx, searchTasks, wg, &recallAcc[i])
	}

	log.Info("Search...")
	startAt = time.Now()
	for i, query := range testData {
		searchTasks <- &searchTask {
			query: query,
			neighborIds: neighborIds[i],
		}
	}
	close(searchTasks)
	wg.Wait()
	d = time.Since(startAt)

	// Search stats
	var totalRecall float64 = 0
	for _, v := range recallAcc {
		totalRecall += v
	}
	log.Infof("Search duration: %s (%.2f queries/s)", d, float64(len(testData)) / d.Seconds())
	log.Infof("Avg recall: %.4f", totalRecall / float64(len(testData)))
}