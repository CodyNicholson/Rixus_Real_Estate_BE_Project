package handlers

import (
	"bytes"
	"fmt"
	"strings"
)

// pdfTextLine represents a line of text on a PDF page.
type pdfTextLine struct {
	text     string
	x, y     float64
	fontSize float64
	bold     bool
	colorR   float64
	colorG   float64
	colorB   float64
}

// pdfRect represents a drawn rectangle (box or rule).
type pdfRect struct {
	x, y, w, h float64
	fill       bool
	stroke     bool
	colorR     float64
	colorG     float64
	colorB     float64
}

// pdfPage represents a single page with drawn elements.
type pdfPage struct {
	lines []pdfTextLine
	rects []pdfRect
}

func newPDFPage() *pdfPage {
	return &pdfPage{
		lines: make([]pdfTextLine, 0),
		rects: make([]pdfRect, 0),
	}
}

func (p *pdfPage) addText(text string, x, y, size float64, bold bool, r, g, b float64) {
	p.lines = append(p.lines, pdfTextLine{
		text:     text,
		x:        x,
		y:        y,
		fontSize: size,
		bold:     bold,
		colorR:   r,
		colorG:   g,
		colorB:   b,
	})
}

func (p *pdfPage) addRect(x, y, w, h float64, fill, stroke bool, r, g, b float64) {
	p.rects = append(p.rects, pdfRect{
		x:      x,
		y:      y,
		w:      w,
		h:      h,
		fill:   fill,
		stroke: stroke,
		colorR: r,
		colorG: g,
		colorB: b,
	})
}

func (p *pdfPage) addRule(x, y, w float64, r, g, b float64) {
	p.addRect(x, y, w, 0.75, true, false, r, g, b)
}

// escapePDFText escapes parentheses and backslashes for PDF literal text strings.
func escapePDFText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `(`, `\(`)
	s = strings.ReplaceAll(s, `)`, `\)`)
	return s
}

// buildPDF compiles multiple pdfPages into a valid PDF-1.4 binary.
func buildPDF(pages []*pdfPage) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")

	offsets := make([]int, 0)
	objNum := 1

	recordObj := func() {
		offsets = append(offsets, buf.Len())
	}

	numPages := len(pages)
	if numPages == 0 {
		pages = []*pdfPage{newPDFPage()}
		numPages = 1
	}

	// Object 1: Catalog -> /Pages 2 0 R
	recordObj()
	fmt.Fprintf(&buf, "%d 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n", objNum)
	objNum++

	// Object 2: Pages dictionary -> list of page object IDs
	// Page objects will be: obj 3 to obj (3 + numPages - 1)
	pageObjStart := objNum + 1 // 3
	var kids strings.Builder
	for i := 0; i < numPages; i++ {
		pObj := pageObjStart + i*2
		kids.WriteString(fmt.Sprintf("%d 0 R ", pObj))
	}

	recordObj()
	fmt.Fprintf(&buf, "%d 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj\n", objNum, strings.TrimSpace(kids.String()), numPages)
	objNum++

	// Fonts:
	// Helvetica (Font 1) and Helvetica-Bold (Font 2)
	font1Obj := pageObjStart + numPages*2
	font2Obj := font1Obj + 1

	// For each page, write Page Object and its Content Stream Object
	for i, page := range pages {
		pObj := pageObjStart + i*2
		cObj := pObj + 1

		// Write Page Object
		recordObj()
		fmt.Fprintf(&buf, "%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 %d 0 R /F2 %d 0 R >> >> /Contents %d 0 R >>\nendobj\n",
			pObj, font1Obj, font2Obj, cObj)

		// Render page content stream
		var streamBuf bytes.Buffer

		// Draw rectangles
		for _, rect := range page.rects {
			if rect.fill {
				fmt.Fprintf(&streamBuf, "%.3f %.3f %.3f rg\n", rect.colorR, rect.colorG, rect.colorB)
				fmt.Fprintf(&streamBuf, "%.2f %.2f %.2f %.2f re f\n", rect.x, rect.y, rect.w, rect.h)
			}
			if rect.stroke {
				fmt.Fprintf(&streamBuf, "%.3f %.3f %.3f RG\n", rect.colorR, rect.colorG, rect.colorB)
				fmt.Fprintf(&streamBuf, "%.2f %.2f %.2f %.2f re s\n", rect.x, rect.y, rect.w, rect.h)
			}
		}

		// Draw text lines
		for _, line := range page.lines {
			fontTag := "/F1"
			if line.bold {
				fontTag = "/F2"
			}
			fmt.Fprintf(&streamBuf, "BT\n")
			fmt.Fprintf(&streamBuf, "%.3f %.3f %.3f rg\n", line.colorR, line.colorG, line.colorB)
			fmt.Fprintf(&streamBuf, "%s %.1f Tf\n", fontTag, line.fontSize)
			fmt.Fprintf(&streamBuf, "%.2f %.2f Td\n", line.x, line.y)
			fmt.Fprintf(&streamBuf, "(%s) Tj\n", escapePDFText(line.text))
			fmt.Fprintf(&streamBuf, "ET\n")
		}

		contentBytes := streamBuf.Bytes()

		// Write Content Stream Object
		recordObj()
		fmt.Fprintf(&buf, "%d 0 obj\n<< /Length %d >>\nstream\n", cObj, len(contentBytes))
		buf.Write(contentBytes)
		buf.WriteString("\nendstream\nendobj\n")
	}

	// Write Font Objects
	recordObj()
	fmt.Fprintf(&buf, "%d 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n", font1Obj)

	recordObj()
	fmt.Fprintf(&buf, "%d 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>\nendobj\n", font2Obj)

	totalObjs := font2Obj

	// Cross-Reference Table
	xrefOffset := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", totalObjs+1)
	buf.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offset)
	}

	// Trailer
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\n", totalObjs+1)
	fmt.Fprintf(&buf, "startxref\n%d\n%%%%EOF\n", xrefOffset)

	return buf.Bytes()
}

// GenerateIllinoisLeasePDF creates a comprehensive Illinois 2026 Residential Lease Agreement PDF.
func GenerateIllinoisLeasePDF() []byte {
	pages := make([]*pdfPage, 0)

	// ================= PAGE 1 =================
	p1 := newPDFPage()

	// Top Banner Box (Navy / Gold Theme)
	p1.addRect(40, 720, 532, 45, true, false, 0.04, 0.08, 0.15) // Dark navy
	p1.addRect(40, 717, 532, 3, true, false, 0.85, 0.65, 0.20)  // Gold accent line

	p1.addText("STATE OF ILLINOIS RESIDENTIAL LEASE AGREEMENT", 50, 742, 13, true, 0.90, 0.72, 0.30)
	p1.addText("Standard Statutory Form - 2026 Edition | Compliant with 765 ILCS 705/ & 765 ILCS 710/", 50, 727, 8, false, 0.70, 0.85, 0.95)

	y := 695.0

	// Section 1: Parties & Premises
	p1.addRect(40, y-18, 532, 18, true, false, 0.93, 0.95, 0.98)
	p1.addText("1. PARTIES, PROPERTY & TERM", 45, y-13, 9.5, true, 0.05, 0.15, 0.30)
	y -= 30

	p1.addText("This Lease Agreement is entered into on ________________________, 2026, by and between:", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 14
	p1.addText("LANDLORD / AGENT: ____________________________________________________________________", 45, y, 8.5, true, 0.1, 0.1, 0.1)
	y -= 14
	p1.addText("TENANT(S): ___________________________________________________________________________", 45, y, 8.5, true, 0.1, 0.1, 0.1)
	y -= 16
	p1.addText("PREMISES ADDRESS: ___________________________________________ Unit/Apt: _____________", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 14
	p1.addText("City: ________________________________, Illinois  ZIP: ______________ County: _____________", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 16
	p1.addText("LEASE TERM: Commencing on ____________________, 2026, and terminating on ____________________, 2027.", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 22

	// Section 2: Rent & Payment Schedule
	p1.addRect(40, y-18, 532, 18, true, false, 0.93, 0.95, 0.98)
	p1.addText("2. RENT PAYMENTS, DUE DATES & LATE FEES (765 ILCS 705/)", 45, y-13, 9.5, true, 0.05, 0.15, 0.30)
	y -= 28

	p1.addText("Monthly Rent of $_________________ is due on the FIRST (1st) day of each calendar month.", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 13
	p1.addText("Payments accepted via: [  ] Online Portal  [  ] ACH / Bank Transfer  [  ] Check / Cashier's Check.", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 13
	p1.addText("LATE FEES (Illinois Law): If rent is not received by the 5th day of the month, a statutory late fee of $20.00", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 12
	p1.addText("or 20% of the monthly rent (whichever is greater under 765 ILCS 705/10) shall be assessed.", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 22

	// Section 3: Security Deposit
	p1.addRect(40, y-18, 532, 18, true, false, 0.93, 0.95, 0.98)
	p1.addText("3. SECURITY DEPOSIT & STATUTORY RETURN (765 ILCS 710/)", 45, y-13, 9.5, true, 0.05, 0.15, 0.30)
	y -= 28

	p1.addText("Tenant has deposited $_________________ as a Security Deposit to secure faithful performance of this Lease.", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 13
	p1.addText("Deposit Bank / Financial Institution: ___________________________________________________________", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 13
	p1.addText("Statutory Return: Under the Illinois Security Deposit Return Act (765 ILCS 710/1), Landlord must deliver", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 12
	p1.addText("an itemized statement of damages within 30 days and return remaining funds within 45 days after vacancy.", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 22

	// Section 4: Utilities Allocation
	p1.addRect(40, y-18, 532, 18, true, false, 0.93, 0.95, 0.98)
	p1.addText("4. UTILITIES & SERVICES ALLOCATION", 45, y-13, 9.5, true, 0.05, 0.15, 0.30)
	y -= 26

	p1.addText("Responsibility for utility services is allocated as follows (check L = Landlord, T = Tenant):", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 14
	p1.addText("[  ] Water / Sewer: ______   [  ] Electric: ______   [  ] Gas / Heating: ______   [  ] Trash Collection: ______", 55, y, 8.5, true, 0.1, 0.1, 0.1)
	y -= 22

	// Section 5: Landlord Access & Maintenance
	p1.addRect(40, y-18, 532, 18, true, false, 0.93, 0.95, 0.98)
	p1.addText("5. MAINTENANCE, REPAIRS & RIGHT OF ENTRY", 45, y-13, 9.5, true, 0.05, 0.15, 0.30)
	y -= 28

	p1.addText("Landlord shall maintain the structural components, plumbing, electrical, and heating systems in good order.", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 13
	p1.addText("Notice of Entry: Landlord shall provide at least 48 hours advance notice before entering for inspections,", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 12
	p1.addText("repairs, or showing to prospective purchasers/tenants, except in emergency cases involving property damage.", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 24

	// Page 1 Footer
	p1.addRule(40, 45, 532, 0.8, 0.8, 0.8)
	p1.addText("Illinois Residential Lease Agreement (2026) | RIXUS-RE Document Repository | Page 1 of 2", 45, 32, 7.5, false, 0.4, 0.4, 0.4)
	p1.addText("Tenant Initials: [ _______ ] [ _______ ]", 440, 32, 8, true, 0.2, 0.2, 0.2)

	pages = append(pages, p1)

	// ================= PAGE 2 =================
	p2 := newPDFPage()

	// Top Banner
	p2.addRect(40, 725, 532, 38, true, false, 0.04, 0.08, 0.15)
	p2.addRect(40, 722, 532, 3, true, false, 0.85, 0.65, 0.20)
	p2.addText("ILLINOIS RESIDENTIAL LEASE AGREEMENT - STATUTORY DISCLOSURES & EXECUTION", 50, 742, 9.5, true, 0.90, 0.72, 0.30)
	p2.addText("Form 2026-IL-RLA | Section 6 through 10", 50, 730, 8, false, 0.70, 0.85, 0.95)

	y = 700.0

	// Section 6: Statutory Disclosures
	p2.addRect(40, y-18, 532, 18, true, false, 0.93, 0.95, 0.98)
	p2.addText("6. MANDATORY ILLINOIS & FEDERAL DISCLOSURES", 45, y-13, 9.5, true, 0.05, 0.15, 0.30)
	y -= 28

	p2.addText("A. Lead-Based Paint: For housing built before 1978, Landlord has provided EPA pamphlet 'Protect Your Family'", 45, y, 8, false, 0.1, 0.1, 0.1)
	y -= 12
	p2.addText("   from Lead in Your Home' and disclosed any known lead hazards. Lead warning acknowledged: [  ] Yes.", 45, y, 8, false, 0.1, 0.1, 0.1)
	y -= 14
	p2.addText("B. Radon Gas Hazard (420 ILCS 46/15): Landlord has disclosed any known radon test results on the premises.", 45, y, 8, false, 0.1, 0.1, 0.1)
	y -= 14
	p2.addText("C. Smoke & Carbon Monoxide Detectors (430 ILCS 135/): Operable smoke and CO alarms are installed.", 45, y, 8, false, 0.1, 0.1, 0.1)
	y -= 12
	p2.addText("   Tenant agrees to test monthly and promptly replace batteries as needed during occupancy.", 45, y, 8, false, 0.1, 0.1, 0.1)
	y -= 22

	// Section 7: Default & 5-Day Notice
	p2.addRect(40, y-18, 532, 18, true, false, 0.93, 0.95, 0.98)
	p2.addText("7. DEFAULT & STATUTORY 5-DAY NOTICE (735 ILCS 5/9-209)", 45, y-13, 9.5, true, 0.05, 0.15, 0.30)
	y -= 28

	p2.addText("In the event of non-payment of rent, Landlord may serve a Statutory 5-Day Written Notice to Quit under", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 13
	p2.addText("Illinois Code of Civil Procedure (735 ILCS 5/9-209). Non-monetary breaches allow 10-day cure notice.", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 22

	// Section 8: Rules & Subletting
	p2.addRect(40, y-18, 532, 18, true, false, 0.93, 0.95, 0.98)
	p2.addText("8. PETS, SMOKING & SUBLETTING", 45, y-13, 9.5, true, 0.05, 0.15, 0.30)
	y -= 28

	p2.addText("Pets: [  ] No pets permitted without written rider.  [  ] Permitted with approved pet fee: $______________", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 13
	p2.addText("Smoking: Cannabis and tobacco smoking strictly prohibited inside the premises and common building areas.", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 13
	p2.addText("Subletting: Tenant shall not assign, sublet, or transfer the premises without prior written consent.", 45, y, 8.5, false, 0.1, 0.1, 0.1)
	y -= 24

	// Section 9: Signatures & Execution
	p2.addRect(40, y-18, 532, 18, true, false, 0.93, 0.95, 0.98)
	p2.addText("9. SIGNATURES & EXECUTION (BINDING LEGAL CONTRACT)", 45, y-13, 9.5, true, 0.05, 0.15, 0.30)
	y -= 26

	p2.addText("IN WITNESS WHEREOF, the parties hereto have executed this Lease Agreement as of the date written above.", 45, y, 8.5, true, 0.1, 0.1, 0.1)
	y -= 30

	// Signature blocks with visual lines
	// Landlord
	p2.addText("LANDLORD / AUTHORIZED AGENT:", 45, y, 8.5, true, 0.05, 0.15, 0.30)
	p2.addText("TENANT 1 (Primary):", 310, y, 8.5, true, 0.05, 0.15, 0.30)
	y -= 35

	p2.addRule(45, y, 220, 0.4, 0.4, 0.4)
	p2.addRule(310, y, 220, 0.4, 0.4, 0.4)
	y -= 12
	p2.addText("Signature                                                  Date", 45, y, 7.5, false, 0.3, 0.3, 0.3)
	p2.addText("Signature                                                  Date", 310, y, 7.5, false, 0.3, 0.3, 0.3)
	y -= 20

	p2.addText("Print Name: ____________________________", 45, y, 8, false, 0.1, 0.1, 0.1)
	p2.addText("Print Name: ____________________________", 310, y, 8, false, 0.1, 0.1, 0.1)
	y -= 35

	// Tenant 2 & Guarantor
	p2.addText("TENANT 2 (Co-Signer / Adult Occupant):", 45, y, 8.5, true, 0.05, 0.15, 0.30)
	p2.addText("GUARANTOR (If Applicable):", 310, y, 8.5, true, 0.05, 0.15, 0.30)
	y -= 35

	p2.addRule(45, y, 220, 0.4, 0.4, 0.4)
	p2.addRule(310, y, 220, 0.4, 0.4, 0.4)
	y -= 12
	p2.addText("Signature                                                  Date", 45, y, 7.5, false, 0.3, 0.3, 0.3)
	p2.addText("Signature                                                  Date", 310, y, 7.5, false, 0.3, 0.3, 0.3)
	y -= 20

	p2.addText("Print Name: ____________________________", 45, y, 8, false, 0.1, 0.1, 0.1)
	p2.addText("Print Name: ____________________________", 310, y, 8, false, 0.1, 0.1, 0.1)

	// Page 2 Footer
	p2.addRule(40, 45, 532, 0.8, 0.8, 0.8)
	p2.addText("Illinois Residential Lease Agreement (2026) | RIXUS-RE Document Repository | Page 2 of 2", 45, 32, 7.5, false, 0.4, 0.4, 0.4)
	p2.addText("[ EXECUTED STATUTORY COPY ]", 420, 32, 8, true, 0.05, 0.5, 0.2)

	pages = append(pages, p2)

	return buildPDF(pages)
}

// GenerateI9FormPDF creates a compliant Form I-9 Employment Eligibility Verification PDF (2026 Edition).
func GenerateI9FormPDF() []byte {
	pages := make([]*pdfPage, 0)

	// ================= PAGE 1: Section 1 =================
	p1 := newPDFPage()

	// Header box
	p1.addRect(40, 720, 532, 45, true, false, 0.08, 0.18, 0.36) // Deep federal blue
	p1.addRect(40, 717, 532, 3, true, false, 0.85, 0.20, 0.20)  // Red federal accent

	p1.addText("Employment Eligibility Verification", 50, 743, 14, true, 1.0, 1.0, 1.0)
	p1.addText("Department of Homeland Security | U.S. Citizenship and Immigration Services", 50, 730, 8.5, false, 0.85, 0.90, 0.98)
	p1.addText("USCIS Form I-9 (2026 Edition)", 420, 743, 9, true, 0.95, 0.85, 0.30)
	p1.addText("OMB No. 1615-0047", 420, 730, 7.5, false, 0.85, 0.90, 0.98)

	y := 695.0

	// Instructions Callout
	p1.addRect(40, y-28, 532, 28, true, false, 0.95, 0.96, 0.98)
	p1.addText("START HERE: Employers must ensure all parts of Form I-9 are properly completed. Do not file with USCIS.", 45, y-10, 7.5, true, 0.1, 0.2, 0.4)
	p1.addText("Anti-Discrimination Notice: It is illegal to discriminate against work-authorized individuals. Employers CANNOT specify", 45, y-20, 7, false, 0.3, 0.3, 0.3)
	p1.addText("which document(s) an employee must present from the Lists of Acceptable Documents.", 45, y-27, 7, false, 0.3, 0.3, 0.3)
	y -= 40

	// Section 1 Box Header
	p1.addRect(40, y-18, 532, 18, true, false, 0.15, 0.25, 0.45)
	p1.addText("Section 1. Employee Information and Attestation (Completed by employee by first day of employment)", 45, y-13, 8.5, true, 1.0, 1.0, 1.0)
	y -= 28

	// Employee Name Fields
	p1.addText("Last Name (Family Name): ________________________  First Name: ______________________  Middle Initial: ____", 45, y, 8, false, 0.1, 0.1, 0.1)
	y -= 16
	p1.addText("Other Last Names Used (if any): ___________________  Address (Street Number and Name): ________________________", 45, y, 8, false, 0.1, 0.1, 0.1)
	y -= 16
	p1.addText("Apt. Number: _________  City or Town: ________________________  State: _______  ZIP Code: ______________", 45, y, 8, false, 0.1, 0.1, 0.1)
	y -= 16
	p1.addText("Date of Birth (mm/dd/yyyy): ______________  U.S. Social Security Number: [ ___ - ___ - ______ ]", 45, y, 8, false, 0.1, 0.1, 0.1)
	y -= 16
	p1.addText("Employee's E-mail Address: ________________________________  Telephone Number: ________________________", 45, y, 8, false, 0.1, 0.1, 0.1)
	y -= 22

	// Attestation Checkboxes
	p1.addRect(40, y-16, 532, 16, true, false, 0.90, 0.93, 0.97)
	p1.addText("I am aware that federal law provides for imprisonment and/or fines for false statements in connection with this form.", 45, y-12, 7.5, true, 0.1, 0.1, 0.1)
	y -= 24

	p1.addText("I attest, under penalty of perjury, that I am (check one of the following boxes):", 45, y, 8, true, 0.1, 0.1, 0.1)
	y -= 14
	p1.addText("[  ] 1. A citizen of the United States", 55, y, 8, false, 0.1, 0.1, 0.1)
	y -= 14
	p1.addText("[  ] 2. A noncitizen national of the United States (see instructions)", 55, y, 8, false, 0.1, 0.1, 0.1)
	y -= 14
	p1.addText("[  ] 3. A lawful permanent resident (Enter USCIS or A-Number): ________________________________________", 55, y, 8, false, 0.1, 0.1, 0.1)
	y -= 14
	p1.addText("[  ] 4. A noncitizen authorized to work until (expiration date, if applicable): ___________________________", 55, y, 8, false, 0.1, 0.1, 0.1)
	y -= 12
	p1.addText("       Form I-94 Admission Number OR Foreign Passport Number: ____________________________________", 55, y, 7.5, false, 0.3, 0.3, 0.3)
	y -= 24

	// Signature of Employee
	p1.addRule(45, y, 320, 0.5, 0.5, 0.5)
	p1.addRule(390, y, 180, 0.5, 0.5, 0.5)
	y -= 12
	p1.addText("Signature of Employee", 45, y, 8, true, 0.1, 0.1, 0.1)
	p1.addText("Today's Date (mm/dd/yyyy)", 390, y, 8, true, 0.1, 0.1, 0.1)
	y -= 28

	// Preparer / Translator Certification
	p1.addRect(40, y-16, 532, 16, true, false, 0.92, 0.94, 0.97)
	p1.addText("Preparer and/or Translator Certification (check one):", 45, y-12, 8, true, 0.1, 0.2, 0.3)
	y -= 24
	p1.addText("[  ] I did not use a preparer or translator.     [  ] A preparer(s) and/or translator(s) assisted the employee.", 45, y, 8, false, 0.1, 0.1, 0.1)
	y -= 16
	p1.addText("Preparer Signature: _________________________________________  Date: ________________________", 45, y, 8, false, 0.1, 0.1, 0.1)
	y -= 14
	p1.addText("Preparer Name: __________________________________ Address: ____________________________________", 45, y, 8, false, 0.1, 0.1, 0.1)

	// Page 1 Footer
	p1.addRule(40, 45, 532, 0.8, 0.8, 0.8)
	p1.addText("Form I-9 2026 Edition | Department of Homeland Security | Employment Eligibility Verification | Page 1 of 2", 45, 32, 7.5, false, 0.4, 0.4, 0.4)

	pages = append(pages, p1)

	// ================= PAGE 2: Section 2 =================
	p2 := newPDFPage()

	// Header box
	p2.addRect(40, 725, 532, 38, true, false, 0.08, 0.18, 0.36)
	p2.addRect(40, 722, 532, 3, true, false, 0.85, 0.20, 0.20)
	p2.addText("Section 2. Employer or Authorized Representative Review and Verification", 50, 743, 11, true, 1.0, 1.0, 1.0)
	p2.addText("Must be completed and signed within 3 business days of employee's first day of employment", 50, 731, 8, false, 0.85, 0.90, 0.98)

	y = 700.0

	p2.addText("Employee Info from Section 1: Last Name: ___________________ First Name: _________________ Citizenship Cat: ____", 45, y, 8, true, 0.1, 0.1, 0.1)
	y -= 20

	// 3 Columns: List A vs List B & C
	colWidth := 170.0
	p2.addRect(40, y-18, colWidth, 18, true, false, 0.90, 0.93, 0.98)
	p2.addText("List A (Identity & Auth)", 45, y-13, 8.5, true, 0.1, 0.2, 0.4)

	p2.addRect(220, y-18, colWidth, 18, true, false, 0.90, 0.93, 0.98)
	p2.addText("List B (Identity Only)", 225, y-13, 8.5, true, 0.1, 0.2, 0.4)

	p2.addRect(400, y-18, colWidth, 18, true, false, 0.90, 0.93, 0.98)
	p2.addText("List C (Employment Auth)", 405, y-13, 8.5, true, 0.1, 0.2, 0.4)
	y -= 26

	// Column content
	p2.addText("Document Title:", 45, y, 7.5, true, 0.2, 0.2, 0.2)
	p2.addText("Document Title:", 225, y, 7.5, true, 0.2, 0.2, 0.2)
	p2.addText("Document Title:", 405, y, 7.5, true, 0.2, 0.2, 0.2)
	y -= 12
	p2.addText("________________________", 45, y, 7.5, false, 0.4, 0.4, 0.4)
	p2.addText("________________________", 225, y, 7.5, false, 0.4, 0.4, 0.4)
	p2.addText("________________________", 405, y, 7.5, false, 0.4, 0.4, 0.4)
	y -= 18

	p2.addText("Issuing Authority:", 45, y, 7.5, true, 0.2, 0.2, 0.2)
	p2.addText("Issuing Authority:", 225, y, 7.5, true, 0.2, 0.2, 0.2)
	p2.addText("Issuing Authority:", 405, y, 7.5, true, 0.2, 0.2, 0.2)
	y -= 12
	p2.addText("________________________", 45, y, 7.5, false, 0.4, 0.4, 0.4)
	p2.addText("________________________", 225, y, 7.5, false, 0.4, 0.4, 0.4)
	p2.addText("________________________", 405, y, 7.5, false, 0.4, 0.4, 0.4)
	y -= 18

	p2.addText("Document Number:", 45, y, 7.5, true, 0.2, 0.2, 0.2)
	p2.addText("Document Number:", 225, y, 7.5, true, 0.2, 0.2, 0.2)
	p2.addText("Document Number:", 405, y, 7.5, true, 0.2, 0.2, 0.2)
	y -= 12
	p2.addText("________________________", 45, y, 7.5, false, 0.4, 0.4, 0.4)
	p2.addText("________________________", 225, y, 7.5, false, 0.4, 0.4, 0.4)
	p2.addText("________________________", 405, y, 7.5, false, 0.4, 0.4, 0.4)
	y -= 18

	p2.addText("Expiration Date (if any):", 45, y, 7.5, true, 0.2, 0.2, 0.2)
	p2.addText("Expiration Date (if any):", 225, y, 7.5, true, 0.2, 0.2, 0.2)
	p2.addText("Expiration Date (if any):", 405, y, 7.5, true, 0.2, 0.2, 0.2)
	y -= 12
	p2.addText("________________________", 45, y, 7.5, false, 0.4, 0.4, 0.4)
	p2.addText("________________________", 225, y, 7.5, false, 0.4, 0.4, 0.4)
	p2.addText("________________________", 405, y, 7.5, false, 0.4, 0.4, 0.4)
	y -= 28

	// Section 2 Attestation Box
	p2.addRect(40, y-18, 532, 18, true, false, 0.92, 0.95, 0.98)
	p2.addText("Certification: I attest, under penalty of perjury, that (1) I have examined the documentation presented by the above-named employee,", 45, y-10, 6.8, true, 0.1, 0.1, 0.1)
	p2.addText("(2) above-listed documentation appears to be genuine and relate to employee, and (3) employee's first day is ___________________.", 45, y-17, 6.8, true, 0.1, 0.1, 0.1)
	y -= 38

	// Employer Signature Block
	p2.addRule(45, y, 220, 0.5, 0.5, 0.5)
	p2.addRule(310, y, 220, 0.5, 0.5, 0.5)
	y -= 12
	p2.addText("Signature of Employer or Authorized Representative", 45, y, 7.5, true, 0.1, 0.1, 0.1)
	p2.addText("Today's Date (mm/dd/yyyy)", 310, y, 7.5, true, 0.1, 0.1, 0.1)
	y -= 22

	p2.addText("Last Name, First Name of Employer Representative: __________________________________________________", 45, y, 7.5, false, 0.1, 0.1, 0.1)
	y -= 16
	p2.addText("Title: ____________________________________ Employer Business or Organization Name: _______________________", 45, y, 7.5, false, 0.1, 0.1, 0.1)
	y -= 16
	p2.addText("Employer Business Address (Street, City, State, ZIP): ______________________________________________________", 45, y, 7.5, false, 0.1, 0.1, 0.1)
	y -= 30

	// Section 3: Reverification & Rehires
	p2.addRect(40, y-16, 532, 16, true, false, 0.94, 0.94, 0.96)
	p2.addText("Section 3. Reverification and Rehires (To be completed and signed by employer or authorized representative)", 45, y-12, 7.5, true, 0.1, 0.1, 0.1)
	y -= 24
	p2.addText("A. New Name (if applicable): ____________________________________ B. Date of Rehire: ______________________", 45, y, 7.5, false, 0.2, 0.2, 0.2)
	y -= 14
	p2.addText("C. Document Title: ______________________ Document Number: _________________ Expiration: _______________", 45, y, 7.5, false, 0.2, 0.2, 0.2)

	// Page 2 Footer
	p2.addRule(40, 45, 532, 0.8, 0.8, 0.8)
	p2.addText("Form I-9 2026 Edition | Department of Homeland Security | Employment Eligibility Verification | Page 2 of 2", 45, 32, 7.5, false, 0.4, 0.4, 0.4)

	pages = append(pages, p2)

	return buildPDF(pages)
}

