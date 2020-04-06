package index

import (
	"io";
	"errors";
	"encoding/binary";
)

type MetadataFieldType uint8

const (
	MetadataFieldType_UINT32 MetadataFieldType = iota
	MetadataFieldType_UINT64
	MetadataFieldType_INT32
	MetadataFieldType_INT64
	MetadataFieldType_FLOAT32
	MetadataFieldType_FLOAT64
	MetadataFieldType_STRING
)

var (
	MetadataKeyTooLongErr error = errors.New("Metadata key too long.")
	MetadataKeyNotFoundInSchemaErr error = errors.New("Metadata key not found in schema")
	MetadataTypeMissmatchErr error = errors.New("Metadata field type missmatch")
)

type MetadataSchema map[string]MetadataFieldType

func (this MetadataSchema) verify(metadata Metadata) error {
	for k, v := range metadata {
		if len(k) > 200 {
			return MetadataKeyTooLongErr
		}

		t, exists := this[k]
		if !exists {
			return MetadataKeyNotFoundInSchemaErr
		}

		switch t {
		case MetadataFieldType_UINT32:
			switch v.(type) {
			case uint32:
			default:
				return MetadataTypeMissmatchErr
			}
		case MetadataFieldType_UINT64:
			switch v.(type) {
			case uint64:
			default:
				return MetadataTypeMissmatchErr
			}
		case MetadataFieldType_INT32:
			switch v.(type) {
			case int32:
			default:
				return MetadataTypeMissmatchErr
			}
		case MetadataFieldType_INT64:
			switch v.(type) {
			case int64:
			default:
				return MetadataTypeMissmatchErr
			}
		case MetadataFieldType_FLOAT32:
			switch v.(type) {
			case float32:
			default:
				return MetadataTypeMissmatchErr
			}
		case MetadataFieldType_FLOAT64:
			switch v.(type) {
			case float64:
			default:
				return MetadataTypeMissmatchErr
			}
		case MetadataFieldType_STRING:
			switch v.(type) {
			case string:
			default:
				return MetadataTypeMissmatchErr
			}
		}
	}
	return nil
}

func (this MetadataSchema) save(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, uint16(len(this))); err != nil {
		return err
	}

	for k, t := range this {
		if err := binary.Write(w, binary.BigEndian, uint8(len(k))); err != nil {
			return err
		}
		if _, err := io.WriteString(w, k); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, t); err != nil {
			return err
		}
	}
	return nil
}

func (this MetadataSchema) load(r io.Reader) error {
	var length uint16
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return err
	}

	for i := 0; i < int(length); i++ {
		var keyLength uint8
		if err := binary.Read(r, binary.BigEndian, &keyLength); err != nil {
			return err
		}
		keyBytes := make([]byte, keyLength)
		if _, err := r.Read(keyBytes); err != nil {
			return err
		}
		key := string(keyBytes)

		var t MetadataFieldType
		if err := binary.Read(r, binary.BigEndian, &t); err != nil {
			return err
		}

		this[key] = t
	}
	return nil
}