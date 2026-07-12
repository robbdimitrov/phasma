import { describe, it, expect } from 'vitest';
import { searchQueryPrefix, stripSearchQueryPrefix } from './searchQuery';

describe('searchQueryPrefix', () => {
	it('returns @ for an @-prefixed query', () => {
		expect(searchQueryPrefix('@alice')).toBe('@');
	});

	it('returns # for a #-prefixed query', () => {
		expect(searchQueryPrefix('#vacation')).toBe('#');
	});

	it('returns null for a plain query', () => {
		expect(searchQueryPrefix('alice')).toBeNull();
	});

	it('returns null for an empty query', () => {
		expect(searchQueryPrefix('')).toBeNull();
	});
});

describe('stripSearchQueryPrefix', () => {
	it('strips a leading @ when prefix is @', () => {
		expect(stripSearchQueryPrefix('@alice', '@')).toBe('alice');
	});

	it('strips a leading # when prefix is #', () => {
		expect(stripSearchQueryPrefix('#vacation', '#')).toBe('vacation');
	});

	it('returns the query unchanged when prefix is null', () => {
		expect(stripSearchQueryPrefix('alice', null)).toBe('alice');
	});
});
