import { describe, it, expect } from 'vitest';
import { createTypeaheadController } from './typeaheadController.svelte';

describe('createTypeaheadController', () => {
	it('select is a no-op when there is no active token', () => {
		const controller = createTypeaheadController();
		expect(controller.select('hello world', 'alice', null)).toBeNull();
	});

	it('select is a no-op when the selection is empty, e.g. Escape', () => {
		const controller = createTypeaheadController();
		controller.handleInput('hi @al', 6);
		expect(controller.select('hi @al', '', null)).toBeNull();
	});

	it('splices a selected mention into the active @token', () => {
		const controller = createTypeaheadController();
		controller.handleInput('hi @al', 6);
		expect(controller.select('hi @al', 'alice', null)).toBe('hi @alice ');
	});

	it('splices a selected hashtag into the active #token', () => {
		const controller = createTypeaheadController();
		controller.handleInput('check #sv', 9);
		expect(controller.select('check #sv', 'svelte', null)).toBe('check #svelte ');
	});

	it('replaces only the active token, keeping trailing text intact', () => {
		const controller = createTypeaheadController();
		controller.handleInput('hi @al and bye', 6);
		expect(controller.select('hi @al and bye', 'alice', null)).toBe('hi @alice  and bye');
	});

	it('clears items and token after a selection', () => {
		const controller = createTypeaheadController();
		controller.handleInput('hi @al', 6);
		controller.select('hi @al', 'alice', null);
		expect(controller.token).toBeNull();
		expect(controller.items).toEqual([]);
	});
});
