// Computed styles that affect text layout and must be copied onto the mirror
// so it wraps text identically to the textarea.
const MIRRORED_STYLES = [
	'boxSizing',
	'width',
	'paddingTop',
	'paddingRight',
	'paddingBottom',
	'paddingLeft',
	'borderTopWidth',
	'borderRightWidth',
	'borderBottomWidth',
	'borderLeftWidth',
	'fontFamily',
	'fontSize',
	'fontWeight',
	'fontStyle',
	'lineHeight',
	'letterSpacing',
	'textTransform'
] as const;

/**
 * Returns the pixel offset, from the textarea's own top edge, at which a
 * dropdown should be positioned to appear directly below the given caret
 * position's line — accounting for wrapping and scroll. Uses the standard
 * mirror-div technique since layout (wrapping) can't be computed from
 * character counts alone with a non-monospace font.
 */
export function getCaretLineTop(el: HTMLTextAreaElement, caret: number): number {
	const style = getComputedStyle(el);

	const mirror = document.createElement('div');
	mirror.style.position = 'absolute';
	mirror.style.visibility = 'hidden';
	mirror.style.top = '0';
	mirror.style.left = '-9999px';
	mirror.style.height = 'auto';
	mirror.style.overflow = 'hidden';
	mirror.style.whiteSpace = 'pre-wrap';
	mirror.style.wordWrap = 'break-word';
	// Border width only affects layout once a border-style is set; the mirror
	// is invisible regardless, so force one for the copied widths to apply.
	mirror.style.borderStyle = 'solid';
	for (const prop of MIRRORED_STYLES) {
		mirror.style[prop] = style[prop];
	}

	mirror.appendChild(document.createTextNode(el.value.slice(0, caret)));
	const marker = document.createElement('span');
	marker.textContent = '.';
	mirror.appendChild(marker);
	mirror.appendChild(document.createTextNode(el.value.slice(caret) || '.'));

	document.body.appendChild(mirror);
	const lineHeight = parseFloat(style.lineHeight) || marker.offsetHeight;
	const top = marker.offsetTop - el.scrollTop + lineHeight;
	document.body.removeChild(mirror);

	return Math.max(0, top);
}
