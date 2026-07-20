import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest';
import { mount, unmount, flushSync, type ComponentProps } from 'svelte';

const { invalidate, resolve, enhance } = vi.hoisted(() => ({
	invalidate: vi.fn(),
	resolve: vi.fn((path: string) => path),
	enhance: vi.fn()
}));

vi.mock('$app/navigation', () => ({ invalidate }));
vi.mock('$app/paths', () => ({ resolve }));
vi.mock('$app/forms', () => ({ enhance }));

import Page from './+page.svelte';

let container: HTMLDivElement;
let instance: object | undefined;
let requestSubmit: ReturnType<typeof vi.spyOn>;

const data = {
	notifications: [
		{
			id: 'n1',
			type: 'like',
			read: false,
			actorUsername: 'alice',
			actorName: 'Alice',
			actorAvatar: null,
			created: new Date('2026-01-01T00:00:00Z')
		}
	],
	nextCursor: null
};

beforeEach(() => {
	vi.clearAllMocks();
	requestSubmit = vi.spyOn(HTMLFormElement.prototype, 'requestSubmit').mockImplementation(() => {});
});

afterEach(() => {
	if (instance) unmount(instance);
	container?.remove();
	requestSubmit.mockRestore();
});

function render(props: ComponentProps<typeof Page>) {
	container = document.createElement('div');
	document.body.appendChild(container);
	instance = mount(Page, { target: container, props });
	flushSync();
	return container;
}

describe('notifications page mount', () => {
	it('submits the markRead form once the page has actually mounted', () => {
		render({ data } as unknown as ComponentProps<typeof Page>);

		expect(requestSubmit).toHaveBeenCalledTimes(1);
	});
});

describe('notifications page load more', () => {
	it('also submits mark-read for a newly loaded page, not just the initial one', async () => {
		const pagedData = { ...data, nextCursor: 'cursor-1' };
		const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
			new Response(
				JSON.stringify({
					items: [{ ...data.notifications[0], id: 'n2' }],
					nextCursor: null
				}),
				{ status: 200 }
			)
		);

		const el = render({ data: pagedData } as unknown as ComponentProps<typeof Page>);
		expect(requestSubmit).toHaveBeenCalledTimes(1);

		el.querySelector('button')?.click();
		await new Promise((resolve) => setImmediate(resolve));

		expect(fetchSpy).toHaveBeenCalled();
		expect(requestSubmit).toHaveBeenCalledTimes(2);

		fetchSpy.mockRestore();
	});
});
