package waveform

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"
	"unsafe"

	"nuclear-valve-vibration-api/internal/model"
	"nuclear-valve-vibration-api/pkg/utils"
)

const (
	MagicNumber    uint16 = 0x5644
	CurrentVersion byte   = 0x01

	MinSamplingRate  uint32 = 100
	MaxSamplingRate  uint32 = 100000
	MinSamplingCount uint32 = 256
	MaxSamplingCount uint32 = 1048576

	HeaderSize = int(unsafe.Sizeof(FileHeader{}))
	CRCSize    = 4
)

var (
	ErrInvalidMagicNumber  = errors.New("invalid magic number")
	ErrInvalidVersion      = errors.New("invalid version")
	ErrInvalidSamplingRate = errors.New("invalid sampling rate")
	ErrInvalidSampleCount  = errors.New("invalid sample count")
	ErrDataTruncated       = errors.New("data truncated")
	ErrCRCVerification     = errors.New("CRC verification failed")
	ErrInvalidDeviceNo     = errors.New("invalid device number")
	ErrInvalidChannelNo    = errors.New("invalid channel number")
)

type FileHeader struct {
	Magic         uint16
	Version       byte
	DeviceNoLen   byte
	SamplingRate  uint32
	SamplingCount uint32
	CollectTime   int64
	ChannelNo     byte
	Reserved      byte
}

type ParsedWaveform struct {
	DeviceNo      string
	SamplingRate  uint32
	SamplingCount uint32
	ChannelNo     uint8
	CollectTime   time.Time
	VibrationData []float64
	CRC           uint32
	RawDataSize   uint32
	RawData       []byte
	WaveformHash  string
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s - %s", e.Field, e.Message)
}

func Parse(data []byte) (*ParsedWaveform, error) {
	if len(data) < HeaderSize+CRCSize {
		return nil, ErrDataTruncated
	}

	header := (*FileHeader)(unsafe.Pointer(&data[0]))

	if header.Magic != MagicNumber {
		return nil, &ValidationError{Field: "MagicNumber", Message: ErrInvalidMagicNumber.Error()}
	}

	if header.Version != CurrentVersion {
		return nil, &ValidationError{Field: "Version", Message: ErrInvalidVersion.Error()}
	}

	if header.DeviceNoLen == 0 || header.DeviceNoLen > 64 {
		return nil, &ValidationError{Field: "DeviceNo", Message: ErrInvalidDeviceNo.Error()}
	}

	deviceNoOffset := HeaderSize
	deviceNoEnd := deviceNoOffset + int(header.DeviceNoLen)
	if deviceNoEnd > len(data)-CRCSize {
		return nil, ErrDataTruncated
	}
	deviceNo := string(data[deviceNoOffset:deviceNoEnd])

	if err := validateSamplingRate(header.SamplingRate); err != nil {
		return nil, err
	}

	if err := validateSamplingCount(header.SamplingCount); err != nil {
		return nil, err
	}

	if header.ChannelNo == 0 {
		return nil, &ValidationError{Field: "ChannelNo", Message: ErrInvalidChannelNo.Error()}
	}

	vibDataOffset := deviceNoEnd
	vibDataSize := int(header.SamplingCount) * 4
	vibDataEnd := vibDataOffset + vibDataSize
	if vibDataEnd > len(data)-CRCSize {
		return nil, ErrDataTruncated
	}

	vibrationData := make([]float64, header.SamplingCount)
	for i := uint32(0); i < header.SamplingCount; i++ {
		offset := vibDataOffset + int(i)*4
		bits := binary.LittleEndian.Uint32(data[offset : offset+4])
		vibrationData[i] = float64(math.Float32frombits(bits))
	}

	crcOffset := len(data) - CRCSize
	expectedCRC := binary.LittleEndian.Uint32(data[crcOffset : crcOffset+CRCSize])
	actualCRC := utils.CRC32(data[:crcOffset])
	if actualCRC != expectedCRC {
		return nil, &ValidationError{Field: "CRC", Message: fmt.Sprintf("%s: expected %d, got %d", ErrCRCVerification, expectedCRC, actualCRC)}
	}

	collectTime := time.Unix(0, header.CollectTime)

	waveformHash := utils.SHA256Hex(data)

	return &ParsedWaveform{
		DeviceNo:      deviceNo,
		SamplingRate:  header.SamplingRate,
		SamplingCount: header.SamplingCount,
		ChannelNo:     header.ChannelNo,
		CollectTime:   collectTime,
		VibrationData: vibrationData,
		CRC:           expectedCRC,
		RawDataSize:   uint32(len(data)),
		RawData:       data,
		WaveformHash:  waveformHash,
	}, nil
}

func Generate(deviceNo string, samplingRate, samplingCount uint32, channelNo uint8, collectTime time.Time, vibrationData []float64) ([]byte, error) {
	if err := validateSamplingRate(samplingRate); err != nil {
		return nil, err
	}
	if err := validateSamplingCount(samplingCount); err != nil {
		return nil, err
	}
	if len(deviceNo) == 0 || len(deviceNo) > 64 {
		return nil, &ValidationError{Field: "DeviceNo", Message: ErrInvalidDeviceNo.Error()}
	}
	if channelNo == 0 {
		return nil, &ValidationError{Field: "ChannelNo", Message: ErrInvalidChannelNo.Error()}
	}
	if uint32(len(vibrationData)) != samplingCount {
		return nil, &ValidationError{Field: "VibrationData", Message: "vibration data length mismatch sampling count"}
	}

	deviceNoBytes := []byte(deviceNo)
	header := FileHeader{
		Magic:         MagicNumber,
		Version:       CurrentVersion,
		DeviceNoLen:   byte(len(deviceNoBytes)),
		SamplingRate:  samplingRate,
		SamplingCount: samplingCount,
		CollectTime:   collectTime.UnixNano(),
		ChannelNo:     channelNo,
		Reserved:      0,
	}

	totalSize := HeaderSize + len(deviceNoBytes) + int(samplingCount)*4 + CRCSize
	data := make([]byte, totalSize)

	headerBytes := unsafe.Slice((*byte)(unsafe.Pointer(&header)), HeaderSize)
	copy(data[0:HeaderSize], headerBytes)

	copy(data[HeaderSize:HeaderSize+len(deviceNoBytes)], deviceNoBytes)

	vibOffset := HeaderSize + len(deviceNoBytes)
	for i, v := range vibrationData {
		offset := vibOffset + i*4
		bits := math.Float32bits(float32(v))
		binary.LittleEndian.PutUint32(data[offset:offset+4], bits)
	}

	crc := utils.CRC32(data[:totalSize-CRCSize])
	binary.LittleEndian.PutUint32(data[totalSize-CRCSize:], crc)

	return data, nil
}

func validateSamplingRate(rate uint32) error {
	if rate < MinSamplingRate || rate > MaxSamplingRate {
		return &ValidationError{
			Field:   "SamplingRate",
			Message: fmt.Sprintf("%s: %d, must be between %d and %d", ErrInvalidSamplingRate, rate, MinSamplingRate, MaxSamplingRate),
		}
	}
	return nil
}

func validateSamplingCount(count uint32) error {
	if count < MinSamplingCount || count > MaxSamplingCount {
		return &ValidationError{
			Field:   "SamplingCount",
			Message: fmt.Sprintf("%s: %d, must be between %d and %d", ErrInvalidSampleCount, count, MinSamplingCount, MaxSamplingCount),
		}
	}
	return nil
}

func (pw *ParsedWaveform) ToModel() *model.Waveform {
	return &model.Waveform{
		DeviceNo:      pw.DeviceNo,
		WaveformHash:  pw.WaveformHash,
		SamplingRate:  pw.SamplingRate,
		SamplingCount: pw.SamplingCount,
		ChannelNo:     pw.ChannelNo,
		CollectTime:   pw.CollectTime,
		RawDataSize:   pw.RawDataSize,
		RawData:       pw.RawData,
		VibrationData: pw.VibrationData,
		CRC:           pw.CRC,
	}
}
