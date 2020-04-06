package index

import (
	"io";
	"encoding/binary";
)

type Metadata map[string]interface{}

func (this Metadata) save(w io.Writer, schema MetadataSchema) error {
	if err := binary.Write(w, binary.BigEndian, uint16(len(this))); err != nil {
		return err
	}
	for k, v := range this {
		if t, exists := schema[k]; exists {
			if err := this.saveKV(w, k, v, t); err != nil {
				return err
			}
		} else {
			return MetadataKeyNotFoundInSchemaErr
		}
	}
	return nil
}

func (this Metadata) load(r io.Reader, schema MetadataSchema) error {
	var length uint16
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return err
	}
	for i := 0; i < int(length); i++ {
		k, v, err := this.loadKV(r, schema)
		if err != nil {
			if err == MetadataKeyNotFoundInSchemaErr {
				continue
			}
			return err
		}	
		this[k] = v
	}
	return nil
}

func (this Metadata) saveKV(w io.Writer, k string, v interface{}, t MetadataFieldType) error {
	if err := binary.Write(w, binary.BigEndian, uint8(len(k))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, k); err != nil {
		return err
	}

	switch t {
	case MetadataFieldType_UINT32:
		return binary.Write(w, binary.BigEndian, v.(uint32))
	case MetadataFieldType_UINT64:
		return binary.Write(w, binary.BigEndian, v.(uint64))
	case MetadataFieldType_INT32:
		return binary.Write(w, binary.BigEndian, v.(int32))
	case MetadataFieldType_INT64:
		return binary.Write(w, binary.BigEndian, v.(int64))
	case MetadataFieldType_FLOAT32:
		return binary.Write(w, binary.BigEndian, v.(float32))
	case MetadataFieldType_FLOAT64:
		return binary.Write(w, binary.BigEndian, v.(float64))
	case MetadataFieldType_STRING:
		if err := binary.Write(w, binary.BigEndian, uint16(len(k))); err != nil {
			return err
		}
		_, err := io.WriteString(w, v.(string))
		return err
	}
	return nil
}

func (this *Metadata) loadKV(r io.Reader, schema MetadataSchema) (string, interface{}, error) {
	var keyLength uint8
	if err := binary.Read(r, binary.BigEndian, &keyLength); err != nil {
		return "", nil, err
	}
	keyBytes := make([]byte, keyLength)
	if _, err := r.Read(keyBytes); err != nil {
		return "", nil, err
	}
	key := string(keyBytes)

	t, exists := schema[key]
	if !exists {
		return "", nil, MetadataKeyNotFoundInSchemaErr
	}

	switch t {
	case MetadataFieldType_UINT32:
		var val uint32
		if err := binary.Read(r, binary.BigEndian, &val); err != nil {
			return key, nil, err
		}
		return key, val, nil
	case MetadataFieldType_UINT64:
		var val uint64
		if err := binary.Read(r, binary.BigEndian, &val); err != nil {
			return key, nil, err
		}
		return key, val, nil
	case MetadataFieldType_INT32:
		var val int32
		if err := binary.Read(r, binary.BigEndian, &val); err != nil {
			return key, nil, err
		}
		return key, val, nil
	case MetadataFieldType_INT64:
		var val int64
		if err := binary.Read(r, binary.BigEndian, &val); err != nil {
			return key, nil, err
		}
		return key, val, nil
	case MetadataFieldType_FLOAT32:
		var val float32
		if err := binary.Read(r, binary.BigEndian, &val); err != nil {
			return key, nil, err
		}
		return key, val, nil
	case MetadataFieldType_FLOAT64:
		var val float64
		if err := binary.Read(r, binary.BigEndian, &val); err != nil {
			return key, nil, err
		}
		return key, val, nil
	case MetadataFieldType_STRING:
		var valLength uint16
		if err := binary.Read(r, binary.BigEndian, &valLength); err != nil {
			return key, nil, err
		}

		valBytes := make([]byte, valLength)
		if _, err := r.Read(valBytes); err != nil {
			return key, nil, err
		}
		return key, string(valBytes), nil
	}

	return "", nil, nil
}