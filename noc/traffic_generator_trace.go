package noc

import (
	"os"
	"log"
	"bufio"
	"strings"
	"strconv"
)

type TraceFileLine struct {
	ThreadId int32
	Pc       int64
	Read     bool
	Ea       int64
}

type TraceTrafficGenerator struct {
	Network              *Network
	PacketInjectionRate  float64
	MaxPackets           int64
	TraceFileName        string
	TraceFileLines       []*TraceFileLine
	CurrentTraceFileLine int
}

func NewTraceTrafficGenerator(network *Network, packetInjectionRate float64, maxPackets int64, traceFileName string) *TraceTrafficGenerator {
	var generator = &TraceTrafficGenerator{
		Network:network,
		PacketInjectionRate:packetInjectionRate,
		MaxPackets:maxPackets,
		TraceFileName:traceFileName,
	}

	traceFile, err := os.Open(traceFileName)
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(traceFile)
	for scanner.Scan() {
		var line = scanner.Text()
		var parts = strings.Split(line, ",")

		if parts[0] == "" {
			continue
		}

		threadId, err := strconv.ParseInt(parts[0], 16, 64)
		if err != nil {
			log.Fatal(err)
		}

		pc, err := strconv.ParseInt(parts[1], 16, 64)
		if err != nil {
			log.Fatal(err)
		}

		var read = parts[2] == "R"

		ea, err := strconv.ParseInt(parts[3], 16, 64)
		if err != nil {
			log.Fatal(err)
		}

		var traceFileLine = &TraceFileLine{
			ThreadId:int32(threadId),
			Pc:pc,
			Read:read,
			Ea:ea,
		}

		generator.TraceFileLines = append(generator.TraceFileLines, traceFileLine)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	traceFile.Close()

	generator.CurrentTraceFileLine = 0

	return generator
}

func (generator *TraceTrafficGenerator) AdvanceOneCycle() {
	if (generator.Network.Driver.CycleAccurateEventQueue().CurrentCycle % 100 == 0 && generator.Network.Driver.CycleAccurateEventQueue().CurrentCycle < 100000000) {
		if generator.CurrentTraceFileLine < len(generator.TraceFileLines) {
			var traceFileLine = generator.TraceFileLines[generator.CurrentTraceFileLine]
			generator.CurrentTraceFileLine += 1

			var src = int(traceFileLine.ThreadId)
			var dest = generator.Network.Config.NumNodes - 1

			var packet = NewDataPacket(generator.Network, src, dest, 16, true, func() {})

			generator.Network.Driver.CycleAccurateEventQueue().Schedule(func() {
				generator.Network.Receive(packet)
			}, 1)
		}
	}
}