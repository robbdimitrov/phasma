import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest';
import { mount, unmount, flushSync, type ComponentProps } from 'svelte';

const { goto, resolve, recordRecentSearch } = vi.hoisted(() => ({
	goto: vi.fn(),
	resolve: vi.fn((path: string) => path),
	recordRecentSearch: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto }));
vi.mock('$app/paths', () => ({ resolve }));
vi.mock('$lib/utils/recentSearch', () => ({ recordRecentSearch }));

import SearchSuggestions from './SearchSuggestions.svelte';

let container: HTMLDivElement;
let instance: object | undefined;

const users = [{ username: 'alice', name: 'Alice', avatar: null }];

beforeEach(() => {
	vi.clearAllMocks();
});

afterEach(() => {
	if (instance) unmount(instance);
	container?.remove();
});

function render(props: ComponentProps<typeof SearchSuggestions>) {
	container = document.createElement('div');
	document.body.appendChild(container);
	instance = mount(SearchSuggestions, { target: container, props });
	flushSync();
	return container;
}

function pressKey(key: string) {
	window.dispatchEvent(new KeyboardEvent('keydown', { key, cancelable: true }));
	flushSync();
}

describe('SearchSuggestions keyboard handling', () => {
	it('does not intercept Enter before the user has navigated with arrow keys', () => {
		render({ users, hashtags: [], onclose: vi.fn() });

		pressKey('Enter');

		expect(goto).not.toHaveBeenCalled();
	});

	it('selects the highlighted suggestion on Enter after ArrowDown navigation', () => {
		render({ users, hashtags: [], onclose: vi.fn() });

		pressKey('ArrowDown');
		pressKey('Enter');

		expect(goto).toHaveBeenCalledWith('/@alice');
	});

	it('calls onclose before navigating so a stale in-flight fetch is discarded', () => {
		const onclose = vi.fn();
		render({ users, hashtags: [], onclose });

		pressKey('ArrowDown');
		pressKey('Enter');

		expect(onclose).toHaveBeenCalled();
	});

	it('records the suggestion when selected via keyboard', () => {
		render({ users, hashtags: [], onclose: vi.fn() });

		pressKey('ArrowDown');
		pressKey('Enter');

		expect(recordRecentSearch).toHaveBeenCalledWith('users', 'alice');
	});
});

describe('SearchSuggestions mouse handling', () => {
	it('records the suggestion and closes the dropdown when a row is clicked', () => {
		const onclose = vi.fn();
		const el = render({ users, hashtags: [], onclose });

		el.querySelector('a')?.dispatchEvent(
			new MouseEvent('click', { bubbles: true, cancelable: true })
		);
		flushSync();

		expect(recordRecentSearch).toHaveBeenCalledWith('users', 'alice');
		expect(onclose).toHaveBeenCalled();
		// A mouse click navigates via the anchor's native href, not goto.
		expect(goto).not.toHaveBeenCalled();
	});
});
