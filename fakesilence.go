package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"gopkg.in/alecthomas/kingpin.v3-unstable"
)

var (
	cli = kingpin.New("fakesilence", "Replaces digital silence with inaudible noise to work around live streaming issues with OGG codecs such as Vorbis, OPUS, FLAC, etc.")

	argSampleRate       = cli.Flag("samplerate", "The audio samplerate").Default("44100").Int()
	argFloat            = cli.Flag("float", "Whether to consider the audio as floating-point instead of integer").Default("false").Bool()
	argBits             = cli.Flag("bits", "The audio bit depth. 64 is only valid for floating-point audio").Default("16").Enum("8", "16", "24", "32", "64")
	argEncoding         = cli.Flag("encoding", "Whether to encode samples little-endian or big-endian").Default("le").Enum("le", "be")
	argChannels         = cli.Flag("channels", "The audio channel count.").Default("2").Int()
	argBufferLength     = cli.Flag("buffer-length", "The length of the buffer to use.").Default("16ms").Duration()
	argSilenceThreshold = cli.Flag("silence-threshold", "Duration of digital silence after which we replace the silence with inaudible noise.").Default("1s").Duration()
)

// Constants for the tiniest possible positive floating point numbers.
const (
	tiniestPositiveFloat32 = 1.175494e-38
	tiniestPositiveFloat64 = 2.2250738585072009e-308
)

func init() {
	kingpin.MustParse(cli.Parse(os.Args[1:]))
	rand.Seed(time.Now().Unix())
}

func generateInaudibleSilence(float bool, bigEndian bool, bytesPerSample int, b []byte) {
	buf := new(bytes.Buffer)
	var byteOrder binary.ByteOrder
	if bigEndian {
		byteOrder = binary.BigEndian
	} else {
		byteOrder = binary.LittleEndian
	}

	for i := 0; i < len(b); i += bytesPerSample {
		switch {
		case !float && bytesPerSample == 1: // 8 bit
			binary.Write(buf, byteOrder, int8(rand.Intn(2)-1))
		case !float && bytesPerSample == 2: // 16 bit
			binary.Write(buf, byteOrder, int16(rand.Intn(2)-1))
		case !float && bytesPerSample == 4: // 32 bit
			binary.Write(buf, byteOrder, int32(rand.Intn(2)-1))
		case float && bytesPerSample == 4: // 32 bit
			binary.Write(buf, byteOrder, float32(rand.Intn(2)-1)*tiniestPositiveFloat32)
		case float && bytesPerSample == 8: // 64 bit
			binary.Write(buf, byteOrder, float64(rand.Intn(2)-1)*tiniestPositiveFloat64)
		default:
			panic("unsupported float/bytesPerSample combination")
		}
	}
	copy(b, buf.Bytes())
}

func main() {
	bigEndian := *argEncoding == "be"
	sampleBits, err := strconv.Atoi(*argBits)
	if err != nil {
		log.Fatal(err)
	}

	bytesPerSample := sampleBits / 8
	bytesPerFrame := *argChannels * bytesPerSample
	samplesPerSecond := *argSampleRate
	frameNanoseconds := float64(time.Second) / float64(*argSampleRate)

	bufferLengthMultiplier := float64(*argBufferLength) / float64(time.Second)
	samplesPerBufferDuration := int(bufferLengthMultiplier * float64(samplesPerSecond))
	bytesPerBufferDuration := samplesPerBufferDuration * bytesPerFrame

	silentNanoseconds := float64(0)

	buf := bufio.NewReaderSize(os.Stdin, bytesPerBufferDuration)
	wbuf := bufio.NewWriterSize(os.Stdout, bytesPerBufferDuration)
	offset := 0
	readBytes := make([]byte, bytesPerFrame)
	generatedFakeSilenceByteCount := uint64(0)
	readByteCount := uint64(0)

	for {
		// read to full frame
		n, err := buf.Read(readBytes[offset:])
		if err == io.EOF {
			// this read returns no data, this is end of file, immediately break out
			break
		}
		readByteCount += uint64(n)
		if n+offset < len(readBytes) {
			// read more bytes to fill buffer
			offset += n
			continue
		}

		// check whether full frame is zeroed (digital silence)
		silentNanoseconds += frameNanoseconds
		for i := 0; i < len(readBytes); i++ {
			if readBytes[i] != 0 {
				// reset silence counter, pass through and move on to next frame
				silentNanoseconds = 0
				break
			}
		}

		// are we above silence threshold?
		silentDuration := time.Duration(silentNanoseconds)
		if silentDuration >= *argSilenceThreshold {
			// replace silence
			generateInaudibleSilence(*argFloat, bigEndian, bytesPerSample, readBytes)
			generatedFakeSilenceByteCount += uint64(len(readBytes))
		}

		if _, err := wbuf.Write(readBytes); err != nil {
			log.Fatal(err)
		}

		// read next frame
		offset = 0
	}

	wbuf.Flush()

	log.Println("Bytes originally read:", humanize.Bytes(readByteCount))
	log.Println("Bytes fake-silenced:  ", humanize.Bytes(generatedFakeSilenceByteCount))
	silencePercentage := float64(generatedFakeSilenceByteCount) / float64(readByteCount)
	log.Printf("Silence percentage:   %.2f%%", 100*silencePercentage)
}
