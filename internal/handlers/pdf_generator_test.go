package handlers

import (
	"bytes"
	"testing"
)

func TestGenerateIllinoisLeasePDF(t *testing.T) {
	pdfBytes := GenerateIllinoisLeasePDF()
	if len(pdfBytes) < 500 {
		t.Fatalf("expected PDF byte slice > 500, got %d", len(pdfBytes))
	}

	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-1.4")) {
		t.Fatalf("expected PDF to start with %%PDF-1.4, got prefix: %q", string(pdfBytes[:8]))
	}

	if !bytes.Contains(pdfBytes, []byte("%%EOF")) {
		t.Fatalf("expected PDF to contain %%%%EOF trailer")
	}

	if !bytes.Contains(pdfBytes, []byte("STATE OF ILLINOIS RESIDENTIAL LEASE AGREEMENT")) {
		t.Fatalf("expected Illinois lease content in stream")
	}

	if !bytes.Contains(pdfBytes, []byte("765 ILCS 710")) {
		t.Fatalf("expected statutory citations in stream")
	}
}

func TestGenerateI9FormPDF(t *testing.T) {
	pdfBytes := GenerateI9FormPDF()
	if len(pdfBytes) < 500 {
		t.Fatalf("expected PDF byte slice > 500, got %d", len(pdfBytes))
	}

	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-1.4")) {
		t.Fatalf("expected PDF to start with %%PDF-1.4, got prefix: %q", string(pdfBytes[:8]))
	}

	if !bytes.Contains(pdfBytes, []byte("%%EOF")) {
		t.Fatalf("expected PDF to contain %%%%EOF trailer")
	}

	if !bytes.Contains(pdfBytes, []byte("Employment Eligibility Verification")) {
		t.Fatalf("expected I-9 content in stream")
	}

	if !bytes.Contains(pdfBytes, []byte("USCIS Form I-9")) {
		t.Fatalf("expected USCIS Form I-9 text in stream")
	}
}

