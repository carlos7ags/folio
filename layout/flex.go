// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

// FlexDirection controls the main axis of the flex container.
type FlexDirection int

const (
	// FlexRow lays out children left-to-right (default).
	FlexRow FlexDirection = iota
	// FlexColumn lays out children top-to-bottom.
	FlexColumn
)

// JustifyContent controls distribution of items along the main axis.
type JustifyContent int

const (
	JustifyFlexStart    JustifyContent = iota // pack toward start (default)
	JustifyFlexEnd                            // pack toward end
	JustifyCenter                             // center along main axis
	JustifySpaceBetween                       // equal space between items
	JustifySpaceAround                        // equal space around items
	JustifySpaceEvenly                        // equal space everywhere
)

// AlignItems controls alignment of items along the cross axis.
type AlignItems int

const (
	CrossAlignStretch AlignItems = iota // stretch to fill cross axis (default)
	CrossAlignStart                     // align to cross-start
	CrossAlignEnd                       // align to cross-end
	CrossAlignCenter                    // center on cross axis
)

// FlexWrap controls whether items wrap to new lines.
type FlexWrap int

const (
	FlexNoWrap FlexWrap = iota // single line (default)
	FlexWrapOn                 // wrap to new lines
)

// FlexItem wraps a child element with flex-specific properties.
type FlexItem struct {
	element        Element
	grow           float64     // flex-grow (default 0)
	shrink         float64     // flex-shrink (default 1)
	basis          float64     // flex-basis in points; 0 means auto
	basisUnit      *UnitValue  // lazy-resolved flex-basis (overrides basis when set)
	alignSelf      *AlignItems // per-item override (nil = use container)
	marginTopAuto  bool        // if true, absorb remaining space before this item (flex column)
	marginLeftAuto bool        // if true, absorb remaining space before this item (flex row)
	marginTop      float64     // vertical margin (can be negative to pull item up)
	marginBottom   float64     // vertical margin below item
	marginLeft     float64     // horizontal margin (negative = extend left beyond parent)
	marginRight    float64     // horizontal margin (negative = extend right beyond parent)
}

// SetMarginTopAuto marks this flex item to absorb remaining vertical space
// before it (CSS margin-top: auto in flex column layout).
func (fi *FlexItem) SetMarginTopAuto() *FlexItem { fi.marginTopAuto = true; return fi }

// SetMarginLeftAuto marks this flex item to absorb remaining horizontal space
// before it (CSS margin-left: auto in flex row layout — pushes item to right).
func (fi *FlexItem) SetMarginLeftAuto() *FlexItem { fi.marginLeftAuto = true; return fi }

// NewFlexItem creates a FlexItem wrapping an element.
func NewFlexItem(elem Element) *FlexItem {
	return &FlexItem{element: elem, shrink: 1}
}

// SetGrow sets the flex-grow factor.
func (fi *FlexItem) SetGrow(v float64) *FlexItem { fi.grow = v; return fi }

// SetShrink sets the flex-shrink factor.
func (fi *FlexItem) SetShrink(v float64) *FlexItem { fi.shrink = v; return fi }

// SetBasis sets the flex-basis in points. 0 means auto (use intrinsic size).
func (fi *FlexItem) SetBasis(v float64) *FlexItem { fi.basis = v; return fi }

// SetBasisUnit sets the flex-basis as a UnitValue, resolved lazily at layout time.
func (fi *FlexItem) SetBasisUnit(u UnitValue) *FlexItem { fi.basisUnit = &u; return fi }

// SetAlignSelf overrides the container's align-items for this item.
func (fi *FlexItem) SetAlignSelf(a AlignItems) *FlexItem { fi.alignSelf = &a; return fi }

// SetMargins sets the margins on a flex item. Negative values extend beyond the parent.
func (fi *FlexItem) SetMargins(top, right, bottom, left float64) *FlexItem {
	fi.marginTop = top
	fi.marginRight = right
	fi.marginBottom = bottom
	fi.marginLeft = left
	return fi
}

// Flex is a container that lays out children using flexbox semantics.
// It implements Element and Measurable.
type Flex struct {
	items                []*FlexItem
	direction            FlexDirection
	justify              JustifyContent
	alignItems           AlignItems
	wrap                 FlexWrap
	rowGap               float64
	columnGap            float64
	padding              Padding
	borders              CellBorders
	background           *Color
	spaceBefore          float64
	heightUnit           *UnitValue // forced height for cross-axis stretch
	spaceAfter           float64
	hasDefiniteCrossSize bool // true when parent constrains cross-axis (e.g. wrapper Div has explicit height)
}

// NewFlex creates an empty flex container.
func NewFlex() *Flex {
	return &Flex{}
}

// Add appends a child element with default flex properties (grow=0, shrink=1, basis=auto).
func (f *Flex) Add(elem Element) *Flex {
	f.items = append(f.items, &FlexItem{element: elem, shrink: 1})
	return f
}

// AddItem appends a FlexItem with explicit flex properties.
func (f *Flex) AddItem(item *FlexItem) *Flex {
	f.items = append(f.items, item)
	return f
}

// SetDirection sets the main axis direction.
func (f *Flex) SetDirection(d FlexDirection) *Flex { f.direction = d; return f }

// SetJustifyContent sets main-axis distribution.
func (f *Flex) SetJustifyContent(j JustifyContent) *Flex { f.justify = j; return f }

// SetAlignItems sets cross-axis alignment for all items.
func (f *Flex) SetAlignItems(a AlignItems) *Flex { f.alignItems = a; return f }

// SetWrap enables or disables wrapping.
func (f *Flex) SetWrap(w FlexWrap) *Flex { f.wrap = w; return f }

// SetGap sets both row and column gap.
func (f *Flex) SetGap(gap float64) *Flex { f.rowGap = gap; f.columnGap = gap; return f }

// SetRowGap sets the gap between wrapped lines.
func (f *Flex) SetRowGap(gap float64) *Flex { f.rowGap = gap; return f }

// SetColumnGap sets the gap between items on the same line.
func (f *Flex) SetColumnGap(gap float64) *Flex { f.columnGap = gap; return f }

// SetPadding sets uniform padding on all sides.
func (f *Flex) SetPadding(p float64) *Flex { f.padding = UniformPadding(p); return f }

// SetPaddingAll sets per-side padding.
func (f *Flex) SetPaddingAll(p Padding) *Flex { f.padding = p; return f }

// SetBorders sets the borders around the container.
func (f *Flex) SetBorders(b CellBorders) *Flex { f.borders = b; return f }

// SetBorder sets the same border on all sides.
func (f *Flex) SetBorder(b Border) *Flex { f.borders = AllBorders(b); return f }

// SetBackground sets the background fill color.
func (f *Flex) SetBackground(c Color) *Flex { f.background = &c; return f }

// ClearBackground removes the background color.
func (f *Flex) ClearBackground() *Flex { f.background = nil; return f }

// SetDefiniteCrossSize marks that the Flex operates within a parent that
// constrains its cross-axis (e.g. a wrapper Div with explicit height).
// This enables cross-axis stretching even when the Flex itself has no heightUnit.
func (f *Flex) SetDefiniteCrossSize(v bool) *Flex { f.hasDefiniteCrossSize = v; return f }

// SetSpaceBefore sets extra vertical space before the container.
func (f *Flex) SetSpaceBefore(pts float64) *Flex { f.spaceBefore = pts; return f }

// SetSpaceAfter sets extra vertical space after the container.
func (f *Flex) SetSpaceAfter(pts float64) *Flex { f.spaceAfter = pts; return f }

// ForceHeight implements HeightSettable. Forces height for cross-axis stretch.
func (f *Flex) ForceHeight(u UnitValue) { f.heightUnit = &u }

// ClearHeightUnit removes the forced height.
func (f *Flex) ClearHeightUnit() {
	f.heightUnit = nil
}

// HasExplicitHeight returns true if the Flex has an explicit CSS height set.
func (f *Flex) HasExplicitHeight() bool { return f.heightUnit != nil }

// GetSpaceBefore returns the extra vertical space before the Flex container.
func (f *Flex) GetSpaceBefore() float64 { return f.spaceBefore }

// GetSpaceAfter returns the extra vertical space after the Flex container.
func (f *Flex) GetSpaceAfter() float64 { return f.spaceAfter }

// Layout implements Element.
func (f *Flex) Layout(maxWidth float64) []Line {
	plan := f.PlanLayout(LayoutArea{Width: maxWidth, Height: 1e9})
	totalH := plan.Consumed
	return []Line{{
		Height:      totalH,
		IsLast:      true,
		SpaceBefore: f.spaceBefore,
		SpaceAfterV: f.spaceAfter,
		divRef: &divLayoutRef{
			div:           nil,
			contentHeight: totalH,
			totalHeight:   totalH,
			innerWidth:    maxWidth - f.padding.Left - f.padding.Right,
			outerWidth:    maxWidth,
		},
	}}
}

// MinWidth implements Measurable.
func (f *Flex) MinWidth() float64 {
	hPad := f.padding.Left + f.padding.Right
	if f.direction == FlexColumn {
		// Column: width is the widest child.
		maxW := 0.0
		for _, item := range f.items {
			if m, ok := item.element.(Measurable); ok {
				if w := m.MinWidth(); w > maxW {
					maxW = w
				}
			}
		}
		return maxW + hPad
	}
	// Row: depends on wrap.
	if f.wrap == FlexWrapOn {
		// Can wrap: min is the widest single item.
		maxW := 0.0
		for _, item := range f.items {
			if m, ok := item.element.(Measurable); ok {
				if w := m.MinWidth(); w > maxW {
					maxW = w
				}
			}
		}
		return maxW + hPad
	}
	// No wrap: all items must fit on one line.
	sum := 0.0
	for _, item := range f.items {
		if m, ok := item.element.(Measurable); ok {
			sum += m.MinWidth()
		}
	}
	n := len(f.items)
	if n > 1 {
		sum += f.columnGap * float64(n-1)
	}
	return sum + hPad
}

// MaxWidth implements Measurable.
func (f *Flex) MaxWidth() float64 {
	hPad := f.padding.Left + f.padding.Right
	if f.direction == FlexColumn {
		maxW := 0.0
		for _, item := range f.items {
			if m, ok := item.element.(Measurable); ok {
				if w := m.MaxWidth(); w > maxW {
					maxW = w
				}
			}
		}
		return maxW + hPad
	}
	// Row: all items on one line.
	sum := 0.0
	for _, item := range f.items {
		if m, ok := item.element.(Measurable); ok {
			sum += m.MaxWidth()
		}
	}
	n := len(f.items)
	if n > 1 {
		sum += f.columnGap * float64(n-1)
	}
	return sum + hPad
}

// PlanLayout implements Element.
func (f *Flex) PlanLayout(area LayoutArea) LayoutPlan {
	if len(f.items) == 0 {
		return LayoutPlan{Status: LayoutFull, Consumed: f.spaceBefore + f.padding.Top + f.padding.Bottom + f.spaceAfter}
	}
	if f.direction == FlexColumn {
		return f.planColumn(area)
	}
	return f.planRow(area)
}

// --- Row direction layout ---

// flexLine groups items that share a single horizontal line.
type flexLine struct {
	items         []*FlexItem
	resolvedSizes []float64 // resolved width per item after grow/shrink
}

func (f *Flex) planRow(area LayoutArea) LayoutPlan {
	innerWidth := area.Width - f.padding.Left - f.padding.Right
	innerHeight := area.Height - f.padding.Top - f.padding.Bottom - f.spaceBefore - f.spaceAfter
	if innerHeight < 0 {
		innerHeight = 0
	}

	// If the flex container has an explicit height, use it to constrain
	// children so that percentage heights resolve against the container,
	// not the remaining page height.
	if f.heightUnit != nil {
		resolvedH := f.heightUnit.Resolve(area.Height)
		innerHeight = resolvedH - f.padding.Top - f.padding.Bottom
		if innerHeight < 0 {
			innerHeight = 0
		}
	}

	// Step 1: Measure intrinsic widths for flex-basis resolution.
	basisWidths := f.resolveRowBasis(innerWidth)

	// Step 2: Partition into flex lines.
	lines := f.partitionRowLines(basisWidths, innerWidth)

	// Determine definite cross-size (when container has explicit height or
	// is constrained by a parent wrapper with explicit height).
	definiteCrossSize := 0.0
	if f.heightUnit != nil || f.hasDefiniteCrossSize {
		definiteCrossSize = innerHeight
	}

	// Step 3-7: Lay out each line.
	var allChildren []PlacedBlock
	curY := f.padding.Top
	allFit := true
	fittedLineCount := 0

	for i, line := range lines {
		if i > 0 {
			curY += f.rowGap
		}
		resolvedWidths := f.resolveGrowShrink(line, innerWidth)
		line.resolvedSizes = resolvedWidths

		remainingCross := innerHeight - (curY - f.padding.Top)

		// First pass: lay out each item to determine content height.
		itemPlans := make([]LayoutPlan, len(line.items))
		lineHeight := 0.0
		for j, item := range line.items {
			plan := item.element.PlanLayout(LayoutArea{Width: resolvedWidths[j], Height: remainingCross})
			itemPlans[j] = plan
			if plan.Consumed > lineHeight {
				lineHeight = plan.Consumed
			}
		}

		// Cross-axis stretch (W3C Flexbox §9.4):
		// The line cross-size is the tallest item. With a definite container
		// height (single-line), it may be the container's inner height instead.
		// Items with align-items:stretch grow to fill the line cross-size,
		// even without a definite container height (stretch to tallest sibling).
		lineCrossSize := lineHeight
		if definiteCrossSize > 0 && (f.wrap == FlexNoWrap || len(lines) == 1) {
			if definiteCrossSize > lineCrossSize {
				lineCrossSize = definiteCrossSize
			}
		}

		// Second pass: re-layout stretch items to match line cross-size.
		for j, item := range line.items {
			if itemPlans[j].Consumed >= lineCrossSize-0.01 {
				continue // already at or above line height
			}
			align := f.alignItems
			if item.alignSelf != nil {
				align = *item.alignSelf
			}
			if align != CrossAlignStretch {
				continue
			}
			hs, ok := item.element.(HeightSettable)
			if !ok || hs.HasExplicitHeight() {
				continue
			}
			// Force the cross-size as an absolute height, re-layout,
			// then clear so the element reverts to content-based sizing.
			hs.ForceHeight(Pt(lineCrossSize))
			itemPlans[j] = item.element.PlanLayout(LayoutArea{
				Width:  resolvedWidths[j],
				Height: lineCrossSize,
			})
			hs.ClearHeightUnit()
		}

		// Check if this line fits.
		if curY-f.padding.Top+lineCrossSize > innerHeight+0.01 && fittedLineCount > 0 {
			allFit = false
			break
		}

		// Position items with justify-content and align-items.
		xOffsets := f.computeJustifyOffsets(resolvedWidths, innerWidth)

		// margin-left: auto — absorb remaining space before this item,
		// pushing it (and all subsequent items) to the right.
		for j, item := range line.items {
			if item.marginLeftAuto {
				// Calculate used space up to this item.
				usedBefore := xOffsets[j]
				usedAfter := 0.0
				for k := j; k < len(line.items); k++ {
					usedAfter += resolvedWidths[k]
					if k > j {
						usedAfter += f.columnGap
					}
				}
				autoSpace := innerWidth - usedBefore - usedAfter
				if autoSpace > 0 {
					for k := j; k < len(line.items); k++ {
						xOffsets[k] += autoSpace
					}
				}
				break // only one auto margin per line
			}
		}

		for j, item := range line.items {
			yOffset := f.computeAlignOffset(item, lineCrossSize, itemPlans[j].Consumed)
			for _, block := range itemPlans[j].Blocks {
				b := block
				b.X += f.padding.Left + xOffsets[j]
				b.Y += curY + yOffset
				allChildren = append(allChildren, b)
			}
		}

		curY += lineCrossSize
		fittedLineCount++
	}

	totalH := curY + f.padding.Bottom

	// Apply explicit height if set (CSS height property).
	if f.heightUnit != nil {
		totalH = f.heightUnit.Resolve(area.Height)
	}

	consumed := f.spaceBefore + totalH + f.spaceAfter

	containerBlock := f.makeContainerBlock(allChildren, totalH, area.Width)

	if allFit {
		return LayoutPlan{Status: LayoutFull, Consumed: consumed, Blocks: []PlacedBlock{containerBlock}}
	}

	// Build overflow with remaining lines' items.
	overflow := f.overflowFrom(fittedLineCount, lines)
	return LayoutPlan{Status: LayoutPartial, Consumed: consumed, Blocks: []PlacedBlock{containerBlock}, Overflow: overflow}
}

// effectiveBasis returns the resolved flex-basis for an item.
// Prefers basisUnit (lazy) over basis (absolute), falling back to 0 (auto).
func (fi *FlexItem) effectiveBasis(available float64) float64 {
	if fi.basisUnit != nil {
		return fi.basisUnit.Resolve(available)
	}
	return fi.basis
}

func (f *Flex) resolveRowBasis(innerWidth float64) []float64 {
	widths := make([]float64, len(f.items))
	for i, item := range f.items {
		if b := item.effectiveBasis(innerWidth); b > 0 {
			widths[i] = b
		} else if m, ok := item.element.(Measurable); ok {
			widths[i] = m.MaxWidth()
		} else {
			widths[i] = measureNaturalWidth(item.element, innerWidth)
		}
	}
	return widths
}

func (f *Flex) partitionRowLines(basisWidths []float64, innerWidth float64) []flexLine {
	if f.wrap == FlexNoWrap || len(f.items) == 0 {
		return []flexLine{{items: f.items, resolvedSizes: basisWidths}}
	}

	var lines []flexLine
	var curItems []*FlexItem
	curWidth := 0.0

	for i, item := range f.items {
		itemW := basisWidths[i]
		gapW := 0.0
		if len(curItems) > 0 {
			gapW = f.columnGap
		}
		if len(curItems) > 0 && curWidth+gapW+itemW > innerWidth {
			lines = append(lines, flexLine{items: curItems})
			curItems = nil
			curWidth = 0
			gapW = 0
		}
		curItems = append(curItems, item)
		curWidth += gapW + itemW
	}
	if len(curItems) > 0 {
		lines = append(lines, flexLine{items: curItems})
	}
	return lines
}

func (f *Flex) resolveGrowShrink(line flexLine, innerWidth float64) []float64 {
	n := len(line.items)
	// Compute basis for this line's items.
	basis := make([]float64, n)
	for i, item := range line.items {
		if b := item.effectiveBasis(innerWidth); b > 0 {
			basis[i] = b
		} else if m, ok := item.element.(Measurable); ok {
			basis[i] = m.MaxWidth()
		} else {
			basis[i] = measureNaturalWidth(item.element, innerWidth)
		}
	}

	totalGap := 0.0
	if n > 1 {
		totalGap = f.columnGap * float64(n-1)
	}
	totalBasis := 0.0
	for _, b := range basis {
		totalBasis += b
	}

	freeSpace := innerWidth - totalGap - totalBasis
	resolved := make([]float64, n)
	copy(resolved, basis)

	if freeSpace > 0 {
		// Distribute to growers.
		totalGrow := 0.0
		for _, item := range line.items {
			totalGrow += item.grow
		}
		if totalGrow > 0 {
			for i, item := range line.items {
				resolved[i] += freeSpace * (item.grow / totalGrow)
			}
		}
	} else if freeSpace < 0 {
		// Shrink.
		totalShrinkScaled := 0.0
		for i, item := range line.items {
			totalShrinkScaled += item.shrink * basis[i]
		}
		if totalShrinkScaled > 0 {
			for i, item := range line.items {
				ratio := (item.shrink * basis[i]) / totalShrinkScaled
				resolved[i] += freeSpace * ratio // freeSpace is negative
				if resolved[i] < 0 {
					resolved[i] = 0
				}
			}
		}
	}
	return resolved
}

func (f *Flex) computeJustifyOffsets(widths []float64, innerWidth float64) []float64 {
	n := len(widths)
	offsets := make([]float64, n)
	totalItemWidth := 0.0
	for _, w := range widths {
		totalItemWidth += w
	}

	switch f.justify {
	case JustifyFlexStart:
		x := 0.0
		for i, w := range widths {
			offsets[i] = x
			x += w + f.columnGap
		}
	case JustifyFlexEnd:
		totalGap := 0.0
		if n > 1 {
			totalGap = f.columnGap * float64(n-1)
		}
		x := innerWidth - totalItemWidth - totalGap
		for i, w := range widths {
			offsets[i] = x
			x += w + f.columnGap
		}
	case JustifyCenter:
		totalGap := 0.0
		if n > 1 {
			totalGap = f.columnGap * float64(n-1)
		}
		x := (innerWidth - totalItemWidth - totalGap) / 2
		for i, w := range widths {
			offsets[i] = x
			x += w + f.columnGap
		}
	case JustifySpaceBetween:
		if n <= 1 {
			offsets[0] = 0
		} else {
			gap := (innerWidth - totalItemWidth) / float64(n-1)
			x := 0.0
			for i, w := range widths {
				offsets[i] = x
				x += w + gap
			}
		}
	case JustifySpaceAround:
		if n == 0 {
			break
		}
		gap := (innerWidth - totalItemWidth) / float64(n)
		x := gap / 2
		for i, w := range widths {
			offsets[i] = x
			x += w + gap
		}
	case JustifySpaceEvenly:
		if n == 0 {
			break
		}
		gap := (innerWidth - totalItemWidth) / float64(n+1)
		x := gap
		for i, w := range widths {
			offsets[i] = x
			x += w + gap
		}
	}
	return offsets
}

func (f *Flex) computeAlignOffset(item *FlexItem, lineSize, itemSize float64) float64 {
	align := f.alignItems
	if item.alignSelf != nil {
		align = *item.alignSelf
	}
	switch align {
	case CrossAlignEnd:
		return lineSize - itemSize
	case CrossAlignCenter:
		return (lineSize - itemSize) / 2
	default: // CrossAlignStretch, CrossAlignStart
		return 0
	}
}

func (f *Flex) overflowFrom(fittedLineCount int, lines []flexLine) *Flex {
	var remaining []*FlexItem
	for i := fittedLineCount; i < len(lines); i++ {
		remaining = append(remaining, lines[i].items...)
	}
	return &Flex{
		items:      remaining,
		direction:  f.direction,
		justify:    f.justify,
		alignItems: f.alignItems,
		wrap:       f.wrap,
		rowGap:     f.rowGap,
		columnGap:  f.columnGap,
		padding:    f.padding,
		borders:    f.borders,
		background: f.background,
		spaceAfter: f.spaceAfter,
	}
}

// --- Column direction layout ---

func (f *Flex) planColumn(area LayoutArea) LayoutPlan {
	innerWidth := area.Width - f.padding.Left - f.padding.Right
	innerHeight := area.Height - f.padding.Top - f.padding.Bottom - f.spaceBefore - f.spaceAfter
	if innerHeight < 0 {
		innerHeight = 0
	}

	// If the flex container has an explicit height, use it to constrain
	// children so that percentage heights resolve against the container.
	if f.heightUnit != nil {
		resolvedH := f.heightUnit.Resolve(area.Height)
		innerHeight = resolvedH - f.padding.Top - f.padding.Bottom
		if innerHeight < 0 {
			innerHeight = 0
		}
	}

	// --- Phase 1: Layout all items at natural height (overflow detection). ---

	type colItemResult struct {
		plan      LayoutPlan
		itemWidth float64
		align     AlignItems
	}
	var results []colItemResult
	curY := f.padding.Top
	remaining := innerHeight
	allFit := true
	fittedCount := 0

	// Track partial/overflow for early returns. We need positioned blocks
	// only for overflow paths; the normal path rebuilds them in phase 3.
	var earlyBlocks []PlacedBlock

	for i, item := range f.items {
		if i > 0 {
			if f.rowGap > remaining {
				allFit = false
				break
			}
			curY += f.rowGap
			remaining -= f.rowGap
		}

		// Apply vertical margin-top (can be negative to pull item up).
		if item.marginTop != 0 {
			curY += item.marginTop
			remaining -= item.marginTop
		}

		// margin-top: auto — absorb all remaining space before this item.
		if item.marginTopAuto && remaining > 0 {
			neededBelow := 0.0
			for j := i; j < len(f.items); j++ {
				if j > i {
					neededBelow += f.rowGap
				}
				plan := f.items[j].element.PlanLayout(LayoutArea{Width: innerWidth, Height: 1e9})
				neededBelow += plan.Consumed
				neededBelow += f.items[j].marginBottom
			}
			autoSpace := remaining - neededBelow
			if autoSpace > 0 {
				curY += autoSpace
				remaining -= autoSpace
			}
		}

		// Resolve item width for cross-axis alignment.
		itemWidth := innerWidth
		align := f.alignItems
		if item.alignSelf != nil {
			align = *item.alignSelf
		}
		if align != CrossAlignStretch {
			if m, ok := item.element.(Measurable); ok {
				itemWidth = m.MaxWidth()
				if itemWidth > innerWidth {
					itemWidth = innerWidth
				}
			}
		}

		// Negative horizontal margins widen the item beyond the parent
		// (CSS: margin: 0 -14px expands element to cancel parent padding).
		if item.marginLeft < 0 || item.marginRight < 0 {
			itemWidth -= item.marginLeft + item.marginRight // subtracting negatives = adding
		}

		plan := item.element.PlanLayout(LayoutArea{Width: itemWidth, Height: remaining})

		switch plan.Status {
		case LayoutFull:
			// Use a small epsilon to avoid floating-point precision issues.
			if plan.Consumed > remaining+0.01 && fittedCount > 0 {
				return f.buildColumnResult(earlyBlocks, curY, area.Width, f.items[i:])
			}
			xOffset := f.columnAlignOffset(align, innerWidth, itemWidth) + item.marginLeft
			for _, block := range plan.Blocks {
				b := block
				b.X += f.padding.Left + xOffset
				b.Y += curY
				earlyBlocks = append(earlyBlocks, b)
			}
			results = append(results, colItemResult{plan, itemWidth, align})
			consumed := plan.Consumed + item.marginBottom
			curY += consumed
			remaining -= consumed
			fittedCount++

		case LayoutPartial:
			xOffset := f.columnAlignOffset(align, innerWidth, itemWidth) + item.marginLeft
			for _, block := range plan.Blocks {
				b := block
				b.X += f.padding.Left + xOffset
				b.Y += curY
				earlyBlocks = append(earlyBlocks, b)
			}
			curY += plan.Consumed
			var overflowItems []*FlexItem
			if plan.Overflow != nil {
				overflowItems = append(overflowItems, &FlexItem{
					element:   plan.Overflow,
					grow:      item.grow,
					shrink:    item.shrink,
					basis:     item.basis,
					basisUnit: item.basisUnit,
				})
			}
			overflowItems = append(overflowItems, f.items[i+1:]...)
			return f.buildColumnResult(earlyBlocks, curY, area.Width, overflowItems)

		case LayoutNothing:
			if fittedCount == 0 {
				return LayoutPlan{Status: LayoutNothing}
			}
			return f.buildColumnResult(earlyBlocks, curY, area.Width, f.items[i:])
		}
	}

	// --- Phase 2: Distribute remaining space to flex-grow items. ---

	if allFit && remaining > 0.01 {
		totalGrow := 0.0
		for _, item := range f.items {
			totalGrow += item.grow
		}
		if totalGrow > 0 {
			for idx, item := range f.items {
				if item.grow <= 0 || idx >= len(results) {
					continue
				}
				extra := remaining * (item.grow / totalGrow)
				newH := results[idx].plan.Consumed + extra
				hs, ok := item.element.(HeightSettable)
				if ok && !hs.HasExplicitHeight() {
					hs.ForceHeight(Pt(newH))
					results[idx].plan = item.element.PlanLayout(LayoutArea{
						Width:  results[idx].itemWidth,
						Height: newH,
					})
					hs.ClearHeightUnit()
				}
			}
		}
	}

	// --- Phase 3: Position items (with justify-content and margin-top:auto). ---

	type colItemRange struct {
		startIdx int
		endIdx   int
		consumed float64
		baseY    float64
	}

	var fittedBlocks []PlacedBlock
	var itemRanges []colItemRange
	curY = f.padding.Top

	for i, r := range results {
		item := f.items[i]
		if i > 0 {
			curY += f.rowGap
		}

		// Apply vertical margin-top (can be negative).
		if item.marginTop != 0 {
			curY += item.marginTop
		}

		// Recalculate margin-top:auto with final sizes.
		if item.marginTopAuto {
			neededBelow := 0.0
			for j := i; j < len(results); j++ {
				if j > i {
					neededBelow += f.rowGap
				}
				neededBelow += results[j].plan.Consumed
				neededBelow += f.items[j].marginBottom
			}
			autoSpace := innerHeight - (curY - f.padding.Top) - neededBelow
			if autoSpace > 0 {
				curY += autoSpace
			}
		}

		startIdx := len(fittedBlocks)
		xOffset := f.columnAlignOffset(r.align, innerWidth, r.itemWidth) + item.marginLeft
		for _, block := range r.plan.Blocks {
			b := block
			b.X += f.padding.Left + xOffset
			b.Y += curY
			fittedBlocks = append(fittedBlocks, b)
		}
		consumed := r.plan.Consumed + item.marginBottom
		itemRanges = append(itemRanges, colItemRange{startIdx, len(fittedBlocks), consumed, curY})
		curY += consumed
	}

	// Apply justify-content for column direction.
	if allFit && f.justify != JustifyFlexStart && len(itemRanges) > 0 {
		heights := make([]float64, len(itemRanges))
		for i, r := range itemRanges {
			heights[i] = r.consumed
		}
		offsets := f.computeColumnJustifyOffsets(heights, innerHeight)
		for i, r := range itemRanges {
			yDelta := f.padding.Top + offsets[i] - r.baseY
			for j := r.startIdx; j < r.endIdx; j++ {
				fittedBlocks[j].Y += yDelta
			}
		}
	}

	totalH := curY + f.padding.Bottom

	// Apply explicit height if set (CSS height property).
	if f.heightUnit != nil {
		totalH = f.heightUnit.Resolve(area.Height)
	}

	consumed := f.spaceBefore + totalH + f.spaceAfter
	containerBlock := f.makeContainerBlock(fittedBlocks, totalH, area.Width)

	if allFit {
		return LayoutPlan{Status: LayoutFull, Consumed: consumed, Blocks: []PlacedBlock{containerBlock}}
	}
	return LayoutPlan{Status: LayoutFull, Consumed: consumed, Blocks: []PlacedBlock{containerBlock}}
}

func (f *Flex) columnAlignOffset(align AlignItems, innerWidth, itemWidth float64) float64 {
	switch align {
	case CrossAlignEnd:
		return innerWidth - itemWidth
	case CrossAlignCenter:
		return (innerWidth - itemWidth) / 2
	default:
		return 0
	}
}

func (f *Flex) buildColumnResult(fittedBlocks []PlacedBlock, curY, areaWidth float64, overflowItems []*FlexItem) LayoutPlan {
	totalH := curY + f.padding.Bottom
	consumed := f.spaceBefore + totalH + f.spaceAfter
	containerBlock := f.makeContainerBlock(fittedBlocks, totalH, areaWidth)
	overflow := &Flex{
		items:      overflowItems,
		direction:  f.direction,
		justify:    f.justify,
		alignItems: f.alignItems,
		wrap:       f.wrap,
		rowGap:     f.rowGap,
		columnGap:  f.columnGap,
		padding:    f.padding,
		borders:    f.borders,
		background: f.background,
		spaceAfter: f.spaceAfter,
	}
	return LayoutPlan{Status: LayoutPartial, Consumed: consumed, Blocks: []PlacedBlock{containerBlock}, Overflow: overflow}
}

// --- Shared helpers ---

func (f *Flex) makeContainerBlock(children []PlacedBlock, totalH, outerWidth float64) PlacedBlock {
	capturedFlex := f
	capturedH := totalH
	capturedW := outerWidth
	return PlacedBlock{
		X: 0, Y: f.spaceBefore, Width: outerWidth, Height: totalH,
		Tag: "Div",
		Draw: func(ctx DrawContext, absX, absTopY float64) {
			bottomY := absTopY - capturedH
			if capturedFlex.background != nil {
				ctx.Stream.SaveState()
				setFillColor(ctx.Stream, *capturedFlex.background)
				ctx.Stream.Rectangle(absX, bottomY, capturedW, capturedH)
				ctx.Stream.Fill()
				ctx.Stream.RestoreState()
			}
			drawCellBorders(ctx.Stream, capturedFlex.borders, absX, bottomY, capturedW, capturedH)
		},
		Children: children,
	}
}

// justifyContent also applies to column direction for Y positioning.
func (f *Flex) computeColumnJustifyOffsets(heights []float64, innerHeight float64) []float64 {
	n := len(heights)
	offsets := make([]float64, n)
	totalItemHeight := 0.0
	for _, h := range heights {
		totalItemHeight += h
	}

	switch f.justify {
	case JustifyFlexStart:
		y := 0.0
		for i, h := range heights {
			offsets[i] = y
			y += h + f.rowGap
		}
	case JustifyFlexEnd:
		totalGap := 0.0
		if n > 1 {
			totalGap = f.rowGap * float64(n-1)
		}
		y := innerHeight - totalItemHeight - totalGap
		for i, h := range heights {
			offsets[i] = y
			y += h + f.rowGap
		}
	case JustifyCenter:
		totalGap := 0.0
		if n > 1 {
			totalGap = f.rowGap * float64(n-1)
		}
		y := (innerHeight - totalItemHeight - totalGap) / 2
		for i, h := range heights {
			offsets[i] = y
			y += h + f.rowGap
		}
	case JustifySpaceBetween:
		if n <= 1 {
			offsets[0] = 0
		} else {
			gap := (innerHeight - totalItemHeight) / float64(n-1)
			y := 0.0
			for i, h := range heights {
				offsets[i] = y
				y += h + gap
			}
		}
	case JustifySpaceAround:
		if n == 0 {
			break
		}
		gap := (innerHeight - totalItemHeight) / float64(n)
		y := gap / 2
		for i, h := range heights {
			offsets[i] = y
			y += h + gap
		}
	case JustifySpaceEvenly:
		if n == 0 {
			break
		}
		gap := (innerHeight - totalItemHeight) / float64(n+1)
		y := gap
		for i, h := range heights {
			offsets[i] = y
			y += h + gap
		}
	}
	return offsets
}
