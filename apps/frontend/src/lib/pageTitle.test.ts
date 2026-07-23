import { describe, expect, it } from 'vitest';
import { pageTitle } from './pageTitle';

describe('pageTitle', () => {
	it('returns the bare app name without a page title', () => {
		expect(pageTitle()).toBe('Phasma');
		expect(pageTitle(null)).toBe('Phasma');
	});

	it('formats page titles with the app suffix', () => {
		expect(pageTitle('Notifications')).toBe('Notifications - Phasma');
	});
});
