// Viewport-relative (`position: fixed`) coordinates for a dropdown that has
// been portalled out of a clipped ancestor. Two placement modes cover the two
// current callers: below a single-line input, or at a specific line inside a
// multi-line textarea (e.g. the caret's wrapped line).
export function createFloatingPosition() {
	let top = $state(0);
	let left = $state(0);

	function placeBelow(el: HTMLElement, gap = 4) {
		const rect = el.getBoundingClientRect();
		top = rect.bottom + gap;
		left = rect.left;
	}

	function placeAtLine(el: HTMLElement, lineTop: number, leftOffset = 0) {
		const rect = el.getBoundingClientRect();
		top = rect.top + lineTop;
		left = rect.left + leftOffset;
	}

	return {
		get top() {
			return top;
		},
		get left() {
			return left;
		},
		placeBelow,
		placeAtLine
	};
}
