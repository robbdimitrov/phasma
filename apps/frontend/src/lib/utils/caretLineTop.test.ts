import { describe, it, expect, afterEach } from 'vitest';
import { getCaretLineTop } from './caretLineTop';

// jsdom does not compute real layout, so offsetTop/offsetHeight are stubbed
// to exercise the arithmetic (and the clamp) deterministically.
function mockOffsets(offsetTop: number, offsetHeight: number) {
	Object.defineProperty(HTMLElement.prototype, 'offsetTop', {
		configurable: true,
		get: () => offsetTop
	});
	Object.defineProperty(HTMLElement.prototype, 'offsetHeight', {
		configurable: true,
		get: () => offsetHeight
	});
}

describe('getCaretLineTop', () => {
	afterEach(() => {
		Reflect.deleteProperty(HTMLElement.prototype, 'offsetTop');
		Reflect.deleteProperty(HTMLElement.prototype, 'offsetHeight');
	});

	it('positions below the caret line using the marker offset and line height', () => {
		mockOffsets(100, 20);
		const el = document.createElement('textarea');
		document.body.appendChild(el);
		el.value = 'hello world';

		expect(getCaretLineTop(el, 5)).toBe(120);
	});

	it('clamps a negative offset (caret scrolled above view) to zero', () => {
		mockOffsets(-500, 20);
		const el = document.createElement('textarea');
		document.body.appendChild(el);
		el.value = 'hello world';

		expect(getCaretLineTop(el, 5)).toBe(0);
	});
});
