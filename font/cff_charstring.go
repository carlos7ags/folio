// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import "encoding/binary"

// Type 2 charstring operator codes (Adobe Tech Note #5177 §3 and §4).
// Only the operators that affect reachability — control flow, hint
// counting, and operand width — need explicit handles here; every
// other operator is treated as "consume the operand stack and
// continue" because charstring drawing semantics do not influence
// which subroutines are called.
const (
	t2OpHstem     = 1
	t2OpVstem     = 3
	t2OpCallsubr  = 10
	t2OpReturn    = 11
	t2OpEscape    = 12
	t2OpEndchar   = 14
	t2OpHstemhm   = 18
	t2OpHintmask  = 19
	t2OpCntrmask  = 20
	t2OpVstemhm   = 23
	t2OpCallgsubr = 29
)

// subrBias returns the offset added to a biased subroutine index per
// TN #5177 §4.7. The Type 2 spec stores subr references as a single
// integer that has already been shifted by this bias; the actual
// INDEX position is `bias + biasedIndex`.
func subrBias(numSubrs int) int {
	switch {
	case numSubrs < 1240:
		return 107
	case numSubrs < 33900:
		return 1131
	default:
		return 32768
	}
}

// charstringWalker discovers the set of global and per-FD local
// subroutines that are transitively reachable from a set of starting
// charstrings. Phase 3 subsetting uses the result to decide which
// subroutines must survive verbatim and which can be replaced with a
// single `return` byte (0x0B).
//
// The walker is intentionally a partial Type 2 interpreter: it tracks
// the operand stack precisely enough to read the integer argument of
// every callsubr/callgsubr (and the implied stem count consumed by
// hintmask / cntrmask) but ignores drawing semantics. If a call's
// operand cannot be determined locally — typically because the
// caller's stack carried the argument across a callsubr boundary —
// the walker falls back to marking every subroutine in the affected
// scope reachable. The subset stays correct in that case; it merely
// fails to shrink as aggressively as a perfect analysis would.
type charstringWalker struct {
	gsubr  *cffIndex
	lsubrs []*cffIndex // per-FD; entries may be nil for FDs with no local subrs

	gReached []bool   // size == gsubr.count (nil when gsubr is empty)
	lReached [][]bool // per FD; nil entries when no local subrs

	allLReached []bool // per FD

	// Recursion depth guard. TN #5177 specifies a max subr nesting
	// depth of 10 levels.
	maxDepth int

	// walkCalls counts every entry into walk(). The visited-check
	// at handleCall ensures recursion is bounded, but tests that
	// would otherwise hang on a regression check this counter
	// directly. Phase 3+ does not consult it on the hot path.
	walkCalls int

	// Pessimistic-fallback flags. When set, every subroutine in the
	// scope is treated as reachable regardless of the per-index slice.
	allGReached bool
}

// newCharstringWalker prepares a walker for a CFF whose global subr
// INDEX is gsubr and whose per-FD local subr INDEXes are lsubrs
// (indexed by FD). Either argument may be nil or empty; the walker
// treats absent indexes as "no subroutines to mark".
func newCharstringWalker(gsubr *cffIndex, lsubrs []*cffIndex) *charstringWalker {
	w := &charstringWalker{
		gsubr:       gsubr,
		lsubrs:      lsubrs,
		allLReached: make([]bool, len(lsubrs)),
		lReached:    make([][]bool, len(lsubrs)),
		maxDepth:    10,
	}
	if gsubr != nil && gsubr.count > 0 {
		w.gReached = make([]bool, gsubr.count)
	}
	for i, ls := range lsubrs {
		if ls != nil && ls.count > 0 {
			w.lReached[i] = make([]bool, ls.count)
		}
	}
	return w
}

// Trace walks bytes (a charstring or subroutine body) in the context
// of FD fd, transitively marking every reachable subroutine.
func (w *charstringWalker) Trace(bytes []byte, fd int) {
	w.walk(bytes, fd, 0)
}

// GsubrReached reports whether global subr i should be retained.
func (w *charstringWalker) GsubrReached(i int) bool {
	if w.allGReached {
		return true
	}
	if i < 0 || i >= len(w.gReached) {
		return false
	}
	return w.gReached[i]
}

// LsubrReached reports whether local subr i in FD fd should be
// retained.
func (w *charstringWalker) LsubrReached(fd, i int) bool {
	if fd < 0 || fd >= len(w.lReached) {
		return false
	}
	if fd < len(w.allLReached) && w.allLReached[fd] {
		return true
	}
	if i < 0 || i >= len(w.lReached[fd]) {
		return false
	}
	return w.lReached[fd][i]
}

// walk interprets a Type 2 charstring well enough to discover
// callsubr / callgsubr targets. depth is the current nesting level;
// the spec caps subr recursion at 10. Stems counted by hstem-family
// operators (and the implicit vstem before the first hintmask) are
// summed so hintmask / cntrmask know how many mask bytes follow.
func (w *charstringWalker) walk(bytes []byte, fd int, depth int) {
	w.walkCalls++
	if depth >= w.maxDepth {
		// Bail out to pessimistic reachability rather than risk
		// missing a real call buried in deeply nested subrs. Phase 3
		// then keeps every subroutine in the affected scope.
		w.allGReached = true
		if fd >= 0 && fd < len(w.allLReached) {
			w.allLReached[fd] = true
		}
		return
	}

	var stack []int64
	stems := 0
	pos := 0
	for pos < len(bytes) {
		b := bytes[pos]
		switch {
		case b == 28:
			if pos+3 > len(bytes) {
				return
			}
			v := int64(int16(binary.BigEndian.Uint16(bytes[pos+1 : pos+3])))
			stack = append(stack, v)
			pos += 3
		case b == 255:
			// 5-byte fixed-point 16.16. We do not need the value
			// because callsubr/callgsubr operands must be integers
			// (TN #5177 §4.7), but we push a slot to keep stack
			// alignment correct for subsequent operators.
			if pos+5 > len(bytes) {
				return
			}
			stack = append(stack, 0)
			pos += 5
		case b >= 32 && b <= 246:
			stack = append(stack, int64(b)-139)
			pos++
		case b >= 247 && b <= 250:
			if pos+1 >= len(bytes) {
				return
			}
			stack = append(stack, int64(b-247)*256+int64(bytes[pos+1])+108)
			pos += 2
		case b >= 251 && b <= 254:
			if pos+1 >= len(bytes) {
				return
			}
			stack = append(stack, -int64(b-251)*256-int64(bytes[pos+1])-108)
			pos += 2
		case b == t2OpEscape:
			// Every 2-byte operator (arithmetic, conditional, flex
			// variants) is irrelevant for reachability; consume it
			// and the operand stack.
			if pos+1 >= len(bytes) {
				return
			}
			pos += 2
			stack = nil
		case b == t2OpCallsubr:
			pos++
			w.handleCall(&stack, fd, false, depth)
		case b == t2OpCallgsubr:
			pos++
			w.handleCall(&stack, fd, true, depth)
		case b == t2OpReturn, b == t2OpEndchar:
			return
		case b == t2OpHstem, b == t2OpVstem, b == t2OpHstemhm, b == t2OpVstemhm:
			// Each stem operator consumes pairs of coordinates. The
			// width-prefix rule (odd stack count → first operand is
			// width) cancels out for stem counting: floor(len/2)
			// gives the right number of stems either way.
			stems += len(stack) / 2
			stack = nil
			pos++
		case b == t2OpHintmask, b == t2OpCntrmask:
			// Per TN #5177 §4.3, hintmask/cntrmask may carry the
			// vstem arguments on the operand stack if the explicit
			// vstem operator was omitted (shortcut form). We
			// therefore add half the remaining stack to the stem
			// count before computing mask-byte width.
			stems += len(stack) / 2
			stack = nil
			maskBytes := (stems + 7) / 8
			if pos+1+maskBytes > len(bytes) {
				return
			}
			pos += 1 + maskBytes
		default:
			// Every other operator (moveto, lineto, curveto, ...)
			// consumes its operands and produces no subr calls.
			stack = nil
			pos++
		}
	}
}

// handleCall reads the biased subr index from the top of stack and
// recurses into the target subr if it is in range. When the stack is
// empty the call's argument was supplied by a caller across a
// callsubr boundary; the walker flips to pessimistic reachability for
// the affected scope rather than try to reconstruct the cross-frame
// state.
func (w *charstringWalker) handleCall(stack *[]int64, fd int, global bool, depth int) {
	if len(*stack) == 0 {
		if global {
			w.allGReached = true
		} else if fd >= 0 && fd < len(w.allLReached) {
			w.allLReached[fd] = true
		}
		return
	}
	biased := (*stack)[len(*stack)-1]
	*stack = (*stack)[:len(*stack)-1]

	var idx *cffIndex
	if global {
		idx = w.gsubr
	} else if fd >= 0 && fd < len(w.lsubrs) {
		idx = w.lsubrs[fd]
	}
	if idx == nil || idx.count == 0 {
		return
	}
	realIdx := int(biased) + subrBias(idx.count)
	if realIdx < 0 || realIdx >= idx.count {
		// Out-of-range biased index. Spec-illegal; do not panic
		// downstream Phase 3 work by marking it.
		return
	}

	if global {
		if w.gReached[realIdx] {
			return
		}
		w.gReached[realIdx] = true
	} else {
		if w.lReached[fd][realIdx] {
			return
		}
		w.lReached[fd][realIdx] = true
	}
	w.walk(idx.Object(realIdx), fd, depth+1)
}
