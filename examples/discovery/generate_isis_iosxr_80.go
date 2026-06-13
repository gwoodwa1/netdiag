package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	podCount    = 8
	nodesPerPod = 10
	defaultDir  = "examples/discovery/isis-iosxr-80-captures"
)

func main() {
	if err := generateCaptures(defaultDir); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func generateCaptures(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for pod := 1; pod <= podCount; pod++ {
		for position := 1; position <= nodesPerPod; position++ {
			name := nodeName(pod, position)
			content := capture(name, pod, position)
			if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(content), 0o644); err != nil {
				return err
			}
		}
	}
	fmt.Printf("generated %d IOS XR IS-IS captures in %s\n", podCount*nodesPerPod, dir)
	return nil
}

func capture(name string, pod, position int) string {
	previousPosition := wrap(position-1, nodesPerPod)
	nextPosition := wrap(position+1, nodesPerPod)
	previousPod := wrap(pod-1, podCount)
	nextPod := wrap(pod+1, podCount)

	return fmt.Sprintf(`RP/0/RP0/CPU0:%s#show isis neighbors
IS-IS CORE neighbors:
System Id      Interface        SNPA           State Holdtime Type IETF-NSF
%-14s Te0/0/0/1        Te0/0/0/0      Up    27       L2   Capable
%-14s Te0/0/0/0        Te0/0/0/1      Up    28       L2   Capable
%-14s Te0/0/0/3        Te0/0/0/2      Up    26       L2   Capable
%-14s Te0/0/0/2        Te0/0/0/3      Up    29       L2   Capable

Total neighbor count: 4
`, name,
		nodeName(pod, previousPosition),
		nodeName(pod, nextPosition),
		nodeName(previousPod, position),
		nodeName(nextPod, position),
	)
}

func nodeName(pod, position int) string {
	return fmt.Sprintf("N55%d-%02d", pod, position)
}

func wrap(value, maximum int) int {
	if value < 1 {
		return maximum
	}
	if value > maximum {
		return 1
	}
	return value
}
