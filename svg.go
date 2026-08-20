package hexdeck

import (
	"bytes"
	"fmt"
	"html"
)

// svgLayout is the fixed geometry of the board image. All sizes are in
// pixels. The canvas size is computed from the board: the width from the
// column count, the height from the longest column. Both are pure
// functions of the state, so the output stays deterministic.
type svgLayout struct {
	headerH, columnGap  int
	columnW, columnTop  int
	cardH, cardGap      int
	cardPad, cardRadius int
	titleH              int
	margin              int
}

// svgPalette is the fixed colour scheme of the board image.
type svgPalette struct {
	bg, header, headerText string
	column, columnText     string
	card, cardBorder       string
	cardText, cardMeta     string
	claim, claimText       string
	comment, commentText   string
}

var svgLayoutDefault = svgLayout{
	headerH: 64, columnGap: 16,
	columnW: 220, columnTop: 80,
	cardH: 64, cardGap: 8,
	cardPad: 10, cardRadius: 6,
	titleH: 20, margin: 16,
}

var svgPaletteDefault = svgPalette{
	bg: "#f6f8fa", header: "#24292f", headerText: "#ffffff",
	column: "#eaeef2", columnText: "#57606a",
	card: "#ffffff", cardBorder: "#d0d7de",
	cardText: "#1f2328", cardMeta: "#57606a",
	claim: "#ddf4ff", claimText: "#0969da",
	comment: "#dafbe1", commentText: "#1a7f37",
}

// RenderSVG renders the board as board.svg — the board image for the
// README. The output is deterministic: same state, same bytes, always.
//
// Layout: a header with the board name and the Updated line, then one
// column per configured column, side by side. Each column shows its
// tickets as cards: the id, the title, and small badges for the claim
// and the comment count. Archived tickets are hidden. A ticket in a
// column that is not in the config renders in a trailing column named
// after the column.
//
// The canvas grows with the board: the width with the column count, the
// height with the longest column. Both are pure functions of the state.
// The render is pure: no timestamps beyond the state's Updated field,
// no external fonts, no random ids. Text is XML-escaped.
func RenderSVG(state BoardState) []byte {
	layout := svgLayoutDefault
	palette := svgPaletteDefault

	columns := append([]string(nil), state.Columns...)
	for _, ticket := range sortedTickets(state) {
		if ticket.Archived {
			continue
		}
		if !contains(columns, ticket.Status) {
			columns = append(columns, ticket.Status)
		}
	}

	width := layout.margin + len(columns)*layout.columnW + (len(columns)-1)*layout.columnGap + layout.margin
	height := layout.columnTop + layout.titleH + 12 + svgCardsArea(state, columns, layout) + layout.margin

	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`+"\n",
		width, height, width, height)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="%s"/>`+"\n", width, height, palette.bg)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="%s"/>`+"\n", width, layout.headerH, palette.header)
	fmt.Fprintf(&b, `<text x="16" y="26" font-family="Helvetica, Arial, sans-serif" font-size="18" font-weight="bold" fill="%s">%s</text>`+"\n",
		palette.headerText, html.EscapeString(state.Name))
	fmt.Fprintf(&b, `<text x="16" y="46" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="%s">%s</text>`+"\n",
		palette.headerText, html.EscapeString(svgUpdatedLine(state)))

	for i, column := range columns {
		x := layout.margin + i*(layout.columnW+layout.columnGap)
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="6" fill="%s"/>`+"\n",
			x, layout.columnTop, layout.columnW, height-layout.columnTop-layout.margin, palette.column)
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-family="Helvetica, Arial, sans-serif" font-size="14" font-weight="bold" fill="%s">%s</text>`+"\n",
			x+layout.cardPad, layout.columnTop+layout.titleH, palette.columnText, html.EscapeString(column))
		y := layout.columnTop + layout.titleH + 12
		for _, ticket := range sortedTickets(state) {
			if ticket.Archived || ticket.Status != column {
				continue
			}
			writeSVGCard(&b, layout, palette, x, y, ticket)
			y += layout.cardH + layout.cardGap
		}
	}
	b.WriteString("</svg>\n")
	return b.Bytes()
}

// svgCardsArea returns the height of the card area: the longest column
// times the card height plus the gaps. Zero when every column is empty.
func svgCardsArea(state BoardState, columns []string, layout svgLayout) int {
	max := 0
	for _, column := range columns {
		n := 0
		for _, ticket := range state.Tickets {
			if !ticket.Archived && ticket.Status == column {
				n++
			}
		}
		if n > max {
			max = n
		}
	}
	if max == 0 {
		return 0
	}
	return max*layout.cardH + (max-1)*layout.cardGap
}

// writeSVGCard writes one ticket card at (x, y).
func writeSVGCard(b *bytes.Buffer, layout svgLayout, palette svgPalette, x, y int, ticket Ticket) {
	fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="%d" rx="%d" fill="%s" stroke="%s"/>`+"\n",
		x, y, layout.columnW, layout.cardH, layout.cardRadius, palette.card, palette.cardBorder)
	fmt.Fprintf(b, `<text x="%d" y="%d" font-family="Helvetica, Arial, sans-serif" font-size="12" font-weight="bold" fill="%s">%s</text>`+"\n",
		x+layout.cardPad, y+layout.cardPad+12, palette.cardText, html.EscapeString(ticket.ID))
	fmt.Fprintf(b, `<text x="%d" y="%d" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="%s">%s</text>`+"\n",
		x+layout.cardPad, y+layout.cardPad+28, palette.cardText, html.EscapeString(ticket.Title))
	badgeX := x + layout.cardPad
	if ticket.ClaimedBy != "" {
		label := "claimed by " + ticket.ClaimedBy
		badgeX = writeSVGBadge(b, layout, palette, badgeX, y+layout.cardPad+36, label, palette.claim, palette.claimText)
	}
	if len(ticket.Comments) > 0 {
		label := fmt.Sprintf("%d comment", len(ticket.Comments))
		if len(ticket.Comments) > 1 {
			label += "s"
		}
		writeSVGBadge(b, layout, palette, badgeX, y+layout.cardPad+36, label, palette.comment, palette.commentText)
	}
}

// writeSVGBadge writes one small pill label and returns the x position
// after it. The pill is 14px high; the text is 10px.
func writeSVGBadge(b *bytes.Buffer, layout svgLayout, palette svgPalette, x, y int, label, fill, textFill string) int {
	text := html.EscapeString(label)
	w := 12 + len(text)*6
	fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="14" rx="7" fill="%s"/>`+"\n", x, y, w, fill)
	fmt.Fprintf(b, `<text x="%d" y="%d" font-family="Helvetica, Arial, sans-serif" font-size="10" fill="%s">%s</text>`+"\n",
		x+6, y+10, textFill, text)
	return x + w + 6
}

// svgUpdatedLine builds the "Updated: <ts> · <counts>" line for the
// header. It matches the board.md Updated line.
func svgUpdatedLine(state BoardState) string {
	return "Updated: " + state.Updated.UTC().Format("2006-01-02T15:04:05Z") + " · " + columnCounts(state)
}
