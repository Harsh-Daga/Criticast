package groundtruth

import (
	"testing"

	"github.com/criticast/criticast/internal/mechanism"
)

func TestFormatParseRoundTrip(t *testing.T) {
	in := Record{Token: "A", Site: SiteMutexLock, Goid: 99, Span: "handler"}
	line, err := in.FormatLine()
	if err != nil {
		t.Fatal(err)
	}
	out, err := ParseLine("prefix " + line)
	if err != nil {
		t.Fatal(err)
	}
	if out.Token != in.Token || out.Site != in.Site || out.Goid != in.Goid {
		t.Fatalf("got %+v want %+v", out, in)
	}
}

func TestFormatLineRequiresFields(t *testing.T) {
	_, err := Record{Token: "A"}.FormatLine()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMechanism(t *testing.T) {
	if Mechanism(SiteWorkerRecv) != mechanism.ChanWorkHandoff {
		t.Fatal()
	}
}
