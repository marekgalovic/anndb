package main

import (
	// "os";
	// "bufio";
	"flag";
	"time";
	"context";
	"io";

	pb "github.com/marekgalovic/anndb/protobuf";
	"github.com/marekgalovic/anndb/math";

	"github.com/satori/go.uuid";
	"google.golang.org/grpc";
	log "github.com/sirupsen/logrus";
)

func main() {
	var serverAddr string
	flag.StringVar(&serverAddr, "server", "", "Server address")
	flag.Parse()

	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	var sentAt time.Time

	datasetManager := pb.NewDatasetManagerClient(conn)
	dataManager := pb.NewDataManagerClient(conn)
	search := pb.NewSearchClient(conn)

	sentAt = time.Now()
	dataset, err := datasetManager.Create(context.Background(), &pb.Dataset {
		Dimension: 32,
		PartitionCount: 1,
		ReplicationFactor: 3,
	})
	if err != nil {
		log.Fatal(err)
	}
	id := uuid.Must(uuid.FromBytes(dataset.GetId()))
	log.Info(id, time.Since(sentAt))


	time.Sleep(2 * time.Second)

	// log.Info("Press ENTER to delete the dataset.")
	// bufio.NewReader(os.Stdin).ReadBytes('\n') 

	// sentAt = time.Now()
	// _, err = datasetManager.Delete(context.Background(), &pb.UUIDRequest {
	// 	Id: id.Bytes(),
	// })
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// log.Info(id, time.Since(sentAt))
	// time.Sleep(100 * time.Millisecond)

	
	// time.Sleep(1 * time.Second)

	// id := uuid.Must(uuid.FromString("e9e1c34f-fa30-4fa4-aa29-492266ceed6d"))

	// dataset, err = datasetManager.Get(context.Background(), &pb.UUIDRequest{Id: id.Bytes()})
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// for _, partition := range dataset.GetPartitions() {
	// 	log.Info(uuid.Must(uuid.FromBytes(partition.GetId())), partition.GetNodeIds())
	// }

	sentAt = time.Now()
	n := 100000
	for i := 0; i < n / 100; i++ {
		RETRY:
		batchItems := make([]*pb.BatchItem, 100)
		for j := 0; j < 100; j++ {
			batchItems[j] = &pb.BatchItem {
				Id: uint64(i * 100 + j),
				Value: math.RandomUniformVector(32),
			}
		}

		resp, err := dataManager.BatchInsert(context.Background(), &pb.BatchRequest {
			DatasetId: id.Bytes(),
			Items: batchItems,
		})
		if err != nil {
			log.Error(err)
			goto RETRY
		}
		for id, errString := range resp.GetErrors() {
			log.Errorf("ID: %d, Err: %s", id, errString)
		}
		if i % 10 == 0 {
			log.Info(i)
		}

		// _, err = dataManager.Insert(context.Background(), &pb.InsertRequest {
		// 	DatasetId: id.Bytes(),
		// 	Id: uint64(i),
		// 	Value: math.RandomUniformVector(32),
		// })
		// if err != nil {
		// 	log.Error(err)
		// 	goto RETRY
		// }
		// if i % 1000 == 0 {
		// 	log.Info(i)
		// }
	}
	log.Infof("Insert %d items: %s, %.2f ops/s", n, time.Since(sentAt), float64(n) / time.Since(sentAt).Seconds())

	// time.Sleep(1 * time.Second)

	// // id := uuid.Must(uuid.FromString("e935acfe-b17a-494a-b315-365fd4fac11f"))

	sentAt = time.Now()
	stream, err := search.Search(context.Background(), &pb.SearchRequest {
		DatasetId: id.Bytes(),
		Query: math.RandomUniformVector(32),
		K: 10,
	})
	if err != nil {
		log.Fatal(err)
	}

	for {
		item, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		log.Info(item.Id, item.Score)
	}

	log.Info(time.Since(sentAt))

	// for i := 0; i < 1000; i++ {
	// 	sentAt = time.Now()
	// 	_, err = dataManager.Remove(context.Background(), &pb.RemoveRequest {
	// 		DatasetId: id.Bytes(),
	// 		Id: uint64(i),
	// 	})
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	log.Info(i, time.Since(sentAt))	
	// }

	// sentAt = time.Now()
	// _, err = dataManager.Remove(context.Background(), &pb.RemoveRequest {
	// 	DatasetId: id.Bytes(),
	// 	Id: 123,
	// })
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// log.Info(time.Since(sentAt))

	// log.Info(client.Get(context.Background(), &pb.UUIDRequest{Id: id.Bytes()}))
}