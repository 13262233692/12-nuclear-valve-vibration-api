package waveform

import (
	"bytes"
	"math"
	"testing"
	"time"
)

func TestParse_SamplingRateAbnormal(t *testing.T) {
	tests := []struct {
		name         string
		samplingRate uint32
		expectError  bool
	}{
		{"zero sampling rate", 0, true},
		{"too low sampling rate", 50, true},
		{"min valid sampling rate", 100, false},
		{"normal sampling rate", 1000, false},
		{"max valid sampling rate", 100000, false},
		{"too high sampling rate", 100001, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			samplingCount := uint32(1024)
			vibrationData := make([]float64, samplingCount)
			for i := range vibrationData {
				if tt.samplingRate > 0 {
					vibrationData[i] = math.Sin(2 * math.Pi * 50 * float64(i) / float64(tt.samplingRate))
				} else {
					vibrationData[i] = 0
				}
			}

			data, err := Generate("VLV-TEST-001", tt.samplingRate, samplingCount, 1, time.Now(), vibrationData)
			if err != nil {
				if !tt.expectError {
					t.Fatalf("Generate failed: %v", err)
				}
				return
			}

			if tt.expectError {
				t.Errorf("expected error for sampling rate %d, got nil", tt.samplingRate)
				return
			}

			parsed, err := Parse(data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if parsed.SamplingRate != tt.samplingRate {
				t.Errorf("expected sampling rate %d, got %d", tt.samplingRate, parsed.SamplingRate)
			}
		})
	}
}

func TestParse_TruncatedWaveform(t *testing.T) {
	samplingRate := uint32(1000)
	samplingCount := uint32(1024)
	vibrationData := make([]float64, samplingCount)
	for i := range vibrationData {
		vibrationData[i] = math.Sin(2 * math.Pi * 50 * float64(i) / float64(samplingRate))
	}

	deviceNo := "VLV-TEST-001"
	fullData, err := Generate(deviceNo, samplingRate, samplingCount, 1, time.Now(), vibrationData)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	headerSize := HeaderSize + len(deviceNo)

	tests := []struct {
		name       string
		truncateAt int
		expectErr  bool
	}{
		{"truncated at magic number", 1, true},
		{"truncated at version", 2, true},
		{"truncated at deviceNoLen", 3, true},
		{"truncated at deviceNo middle", headerSize - 5, true},
		{"truncated at samplingRate", headerSize + 1, true},
		{"truncated at samplingCount", headerSize + 5, true},
		{"truncated at collectTime", headerSize + 9, true},
		{"truncated at channelNo", headerSize + 16, true},
		{"truncated at vibration data start", headerSize + 20, true},
		{"truncated at vibration data middle", headerSize + 20 + 100*4, true},
		{"truncated at CRC start", len(fullData) - 3, true},
		{"truncated at CRC middle", len(fullData) - 1, true},
		{"complete data", len(fullData), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncatedData := fullData[:tt.truncateAt]
			_, err := Parse(truncatedData)

			if tt.expectErr {
				if err == nil {
					t.Error("expected truncation error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestParse_DeviceNoLength(t *testing.T) {
	tests := []struct {
		name        string
		deviceNo    string
		expectError bool
	}{
		{"empty device no", "", true},
		{"too long device no", "VLV-TEST-001-TOO-LONG-DEVICE-NUMBER-0000000000000000000000000000000000000", true},
		{"valid short device no", "V1", false},
		{"valid normal device no", "VLV-TEST-001", false},
		{"max valid length 64", "VLV-TEST-DEVICE-NO-MAX-LENGTH-0012345678901234567890123456789012", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			samplingRate := uint32(1000)
			samplingCount := uint32(512)
			vibrationData := make([]float64, samplingCount)
			for i := range vibrationData {
				vibrationData[i] = 0.1 * math.Sin(2*math.Pi*50*float64(i)/float64(samplingRate))
			}

			data, err := Generate(tt.deviceNo, samplingRate, samplingCount, 1, time.Now(), vibrationData)
			if err != nil {
				if !tt.expectError {
					t.Fatalf("Generate failed: %v", err)
				}
				return
			}

			if tt.expectError {
				t.Error("expected generation error, got nil")
				return
			}

			parsed, err := Parse(data)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if parsed.DeviceNo != tt.deviceNo {
				t.Errorf("expected device no %s, got %s", tt.deviceNo, parsed.DeviceNo)
			}
		})
	}
}

func TestParse_CRCValidation(t *testing.T) {
	samplingRate := uint32(1000)
	samplingCount := uint32(512)
	vibrationData := make([]float64, samplingCount)
	for i := range vibrationData {
		vibrationData[i] = 0.1 * math.Sin(2*math.Pi*50*float64(i)/float64(samplingRate))
	}

	data, err := Generate("VLV-TEST-001", samplingRate, samplingCount, 1, time.Now(), vibrationData)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = Parse(data)
	if err != nil {
		t.Fatalf("Parse with valid CRC failed: %v", err)
	}

	corruptedData := make([]byte, len(data))
	copy(corruptedData, data)
	corruptedData[len(corruptedData)-4] ^= 0xFF

	_, err = Parse(corruptedData)
	if err == nil {
		t.Error("expected CRC error for corrupted data, got nil")
	}
}

func TestParse_SamplingCount(t *testing.T) {
	tests := []struct {
		name          string
		samplingCount uint32
		expectError   bool
	}{
		{"zero count", 0, true},
		{"too low count", 100, true},
		{"min valid count", 256, false},
		{"normal count", 1024, false},
		{"max valid count", 1048576, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			samplingRate := uint32(1000)
			vibrationData := make([]float64, tt.samplingCount)
			for i := range vibrationData {
				vibrationData[i] = math.Sin(2 * math.Pi * 50 * float64(i) / float64(samplingRate))
			}

			data, err := Generate("VLV-TEST-001", samplingRate, tt.samplingCount, 1, time.Now(), vibrationData)
			if err != nil {
				if !tt.expectError {
					t.Fatalf("Generate failed: %v", err)
				}
				return
			}

			if tt.expectError {
				t.Errorf("expected error for sampling count %d, got nil", tt.samplingCount)
				return
			}

			parsed, err := Parse(data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if parsed.SamplingCount != tt.samplingCount {
				t.Errorf("expected sampling count %d, got %d", tt.samplingCount, parsed.SamplingCount)
			}

			if len(parsed.VibrationData) != int(tt.samplingCount) {
				t.Errorf("expected vibration data length %d, got %d", tt.samplingCount, len(parsed.VibrationData))
			}
		})
	}
}

func TestGenerateAndParse_RoundTrip(t *testing.T) {
	samplingRate := uint32(10000)
	samplingCount := uint32(4096)
	deviceNo := "VLV-NUCLEAR-001"
	channelNo := uint8(2)
	collectTime := time.Now().Truncate(time.Nanosecond)

	vibrationData := make([]float64, samplingCount)
	for i := range vibrationData {
		t := float64(i) / float64(samplingRate)
		vibrationData[i] = 0.1*math.Sin(2*math.Pi*50*t) + 0.05*math.Sin(2*math.Pi*100*t)
	}

	data, err := Generate(deviceNo, samplingRate, samplingCount, channelNo, collectTime, vibrationData)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if parsed.DeviceNo != deviceNo {
		t.Errorf("DeviceNo mismatch: expected %s, got %s", deviceNo, parsed.DeviceNo)
	}
	if parsed.SamplingRate != samplingRate {
		t.Errorf("SamplingRate mismatch: expected %d, got %d", samplingRate, parsed.SamplingRate)
	}
	if parsed.SamplingCount != samplingCount {
		t.Errorf("SamplingCount mismatch: expected %d, got %d", samplingCount, parsed.SamplingCount)
	}
	if parsed.ChannelNo != channelNo {
		t.Errorf("ChannelNo mismatch: expected %d, got %d", channelNo, parsed.ChannelNo)
	}

	if !parsed.CollectTime.Equal(collectTime) {
		t.Errorf("CollectTime mismatch: expected %v, got %v", collectTime, parsed.CollectTime)
	}

	if len(parsed.VibrationData) != len(vibrationData) {
		t.Fatalf("VibrationData length mismatch: expected %d, got %d", len(vibrationData), len(parsed.VibrationData))
	}

	for i := range vibrationData {
		diff := math.Abs(parsed.VibrationData[i] - vibrationData[i])
		if diff > 1e-7 {
			t.Errorf("VibrationData mismatch at index %d: expected %f, got %f (diff %f)",
				i, vibrationData[i], parsed.VibrationData[i], diff)
			break
		}
	}

	if parsed.CRC == 0 {
		t.Error("CRC should not be zero")
	}

	if parsed.WaveformHash == "" {
		t.Error("WaveformHash should not be empty")
	}

	if !bytes.Equal(parsed.RawData, data) {
		t.Error("RawData mismatch")
	}
}

func TestParse_InvalidMagicNumber(t *testing.T) {
	samplingRate := uint32(1000)
	samplingCount := uint32(512)
	vibrationData := make([]float64, samplingCount)

	data, err := Generate("VLV-TEST-001", samplingRate, samplingCount, 1, time.Now(), vibrationData)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	badData := make([]byte, len(data))
	copy(badData, data)
	badData[0] = 0x00
	badData[1] = 0x00

	_, err = Parse(badData)
	if err == nil {
		t.Error("expected magic number error, got nil")
	}
}

func TestParse_InvalidChannel(t *testing.T) {
	samplingRate := uint32(1000)
	samplingCount := uint32(512)
	vibrationData := make([]float64, samplingCount)

	_, err := Generate("VLV-TEST-001", samplingRate, samplingCount, 0, time.Now(), vibrationData)
	if err == nil {
		t.Error("expected channel number error, got nil")
	}
}
