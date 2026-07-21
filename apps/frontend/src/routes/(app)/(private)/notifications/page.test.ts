import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest';
import { mount, unmount, flushSync, type ComponentProps } from 'svelte';

const { invalidate, resolve } = vi.hoisted(() => ({
	invalidate: vi.fn(),
	resolve: vi.fn((path: string) => path)
}));

vi.mock('$app/navigation', () => ({ invalidate }));
vi.mock('$app/paths', () => ({ resolve }));

import Page from './+page.svelte';

let container: HTMLDivElement;
let instance: object | undefined;
let fetchSpy: ReturnType<typeof vi.spyOn>;

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
	fetchSpy = vi
		.spyOn(globalThis, 'fetch')
		.mockResolvedValue(
			new Response(JSON.stringify({ items: [], nextCursor: null }), { status: 200 })
		);
});

afterEach(() => {
	if (instance) unmount(instance);
	container?.remove();
	fetchSpy.mockRestore();
});

function render(props: ComponentProps<typeof Page>) {
	container = document.createElement('div');
	document.body.appendChild(container);
	instance = mount(Page, { target: container, props });
	flushSync();
	return container;
}

describe('notifications page mount', () => {
	it('POSTs the unread ids to the same-origin route once the page has actually mounted', () => {
		render({ data } as unknown as ComponentProps<typeof Page>);

		expect(fetchSpy).toHaveBeenCalledWith(
			'/notifications',
			expect.objectContaining({ method: 'POST', body: JSON.stringify({ ids: ['n1'] }) })
		);
	});
});

describe('notifications page load more', () => {
	it('also POSTs mark-read for a newly loaded page, not just the initial one', async () => {
		const pagedData = { ...data, nextCursor: 'cursor-1' };
		fetchSpy.mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
			const url = typeof input === 'string' ? input : input.toString();
			if (init?.method === 'POST') return new Response(null, { status: 200 });
			if (url.includes('cursor=cursor-1')) {
				return new Response(
					JSON.stringify({ items: [{ ...data.notifications[0], id: 'n2' }], nextCursor: null }),
					{ status: 200 }
				);
			}
			return new Response(JSON.stringify({ items: [], nextCursor: null }), { status: 200 });
		});

		const el = render({ data: pagedData } as unknown as ComponentProps<typeof Page>);
		expect(fetchSpy).toHaveBeenCalledWith(
			'/notifications',
			expect.objectContaining({ method: 'POST', body: JSON.stringify({ ids: ['n1'] }) })
		);

		el.querySelector('button')?.click();
		await new Promise((resolve) => setImmediate(resolve));

		expect(fetchSpy).toHaveBeenCalledWith(
			'/notifications',
			expect.objectContaining({ method: 'POST', body: JSON.stringify({ ids: ['n2'] }) })
		);
	});
});
