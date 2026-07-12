import { describe, it, expect } from 'vitest';
import { createFloatingPosition } from './floatingPosition.svelte';

function mockRect(el: HTMLElement, rect: Partial<DOMRect>) {
	el.getBoundingClientRect = () =>
		({
			top: 0,
			left: 0,
			bottom: 0,
			right: 0,
			width: 0,
			height: 0,
			x: 0,
			y: 0,
			toJSON() {},
			...rect
		}) as DOMRect;
}

describe('createFloatingPosition', () => {
	it('placeBelow anchors under the element with a default gap', () => {
		const el = document.createElement('input');
		mockRect(el, { left: 10, bottom: 50 });

		const pos = createFloatingPosition();
		pos.placeBelow(el);

		expect(pos.left).toBe(10);
		expect(pos.top).toBe(54);
	});

	it('placeBelow accepts a custom gap', () => {
		const el = document.createElement('input');
		mockRect(el, { left: 10, bottom: 50 });

		const pos = createFloatingPosition();
		pos.placeBelow(el, 0);

		expect(pos.top).toBe(50);
	});

	it('placeAtLine offsets from the element top by the given line/left offsets', () => {
		const el = document.createElement('textarea');
		mockRect(el, { top: 100, left: 20 });

		const pos = createFloatingPosition();
		pos.placeAtLine(el, 30, 24);

		expect(pos.top).toBe(130);
		expect(pos.left).toBe(44);
	});
});
