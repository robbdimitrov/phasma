import { describe, it, expect } from 'vitest';
import { activeToken } from './activeToken';

describe('activeToken', () => {
	it('returns token when caret is inside a @mention', () => {
		const result = activeToken('hello @alice', 12);
		expect(result).toEqual({ trigger: '@', query: 'alice', start: 6, end: 12 });
	});

	it('returns token when caret is inside a #hashtag', () => {
		const result = activeToken('check #svelte', 13);
		expect(result).toEqual({ trigger: '#', query: 'svelte', start: 6, end: 13 });
	});

	it('returns null when caret is not after a trigger', () => {
		const result = activeToken('hello world', 11);
		expect(result).toBeNull();
	});

	it('returns null when trigger is mid-word (no preceding whitespace)', () => {
		const result = activeToken('foo@bar', 7);
		expect(result).toBeNull();
	});

	it('returns token when trigger is at the start of the string', () => {
		const result = activeToken('@alice', 6);
		expect(result).toEqual({ trigger: '@', query: 'alice', start: 0, end: 6 });
	});

	it('returns token when trigger follows a newline', () => {
		const result = activeToken('line one\n@alice', 15);
		expect(result).toEqual({ trigger: '@', query: 'alice', start: 9, end: 15 });
	});

	it('returns empty query when caret is immediately after trigger', () => {
		const result = activeToken('@', 1);
		expect(result).toEqual({ trigger: '@', query: '', start: 0, end: 1 });
	});

	it('returns null for # trigger mid-word', () => {
		const result = activeToken('foo#bar', 7);
		expect(result).toBeNull();
	});

	it('returns null when a #hashtag query contains a dot (backend allows only [A-Za-z0-9_])', () => {
		const result = activeToken('check #sv.elte', 14);
		expect(result).toBeNull();
	});

	it('allows a dot in an @mention query (usernames allow it)', () => {
		const result = activeToken('hello @al.ice', 13);
		expect(result).toEqual({ trigger: '@', query: 'al.ice', start: 6, end: 13 });
	});
});
