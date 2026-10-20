package gpu

import (
	"bufio"
	"strings"
	"testing"

	"github.com/dhruvinsh/panoptes/internal/frame"
)

func TestParseNvidiaSMI(t *testing.T) {
	g := ParseNvidiaSMI("NVIDIA GeForce RTX 3070, 12, 48, 1200, 8192, 45.20\n")
	if s := string(frame.Encode(g)); s != `{"name":"RTX 3070","pct":12,"temp":48,"mem_used":1200,"mem_total":8192,"power":45.2}`+"\n" {
		t.Fatalf("got %s", s)
	}
	g = ParseNvidiaSMI("NVIDIA GeForce GTX 1050, 3, 40, 100, 2048, [N/A]\n")
	if g.Has("power") || g.Value("pct") != 3 {
		t.Fatalf("got %s", frame.Encode(g))
	}
	if ParseNvidiaSMI("garbage\n") != nil {
		t.Fatal("expected nil")
	}
}

func TestShortenName(t *testing.T) {
	for in, want := range map[string]string{
		"NVIDIA GeForce RTX 2060 SUPER":      "RTX 2060 SUPER",
		"NVIDIA GeForce RTX 4070 Laptop GPU": "RTX 4070",
		"AMD Radeon RX 6800 XT":              "RX 6800 XT",
	} {
		if got := ShortenName(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestScanPCIIDs(t *testing.T) {
	ids := "# comment\n1002  Advanced Micro Devices, Inc. [AMD/ATI]\n\t73bf  Navi 21 [Radeon RX 6800/6800 XT / 6900 XT]\n\t\t1002 0e3a  sub\n10de  NVIDIA\n\t73bf  wrong\n"
	if got := scanPCIIDs(bufio.NewScanner(strings.NewReader(ids)), "1002", "73bf"); got != "Radeon RX 6800/6800 XT / 6900 XT" {
		t.Fatalf("got %q", got)
	}
}
