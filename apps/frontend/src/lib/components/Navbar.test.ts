import { describe, it, expect, afterEach, vi } from 'vitest';
import { mount, unmount, flushSync, type ComponentProps } from 'svelte';
import type { User } from '$lib/types';

const { resolve } = vi.hoisted(() => ({
	resolve: vi.fn((path: string) => path)
}));

vi.mock('$app/paths', () => ({ resolve }));
vi.mock('$app/state', () => ({ page: { url: new URL('http://localhost/feed') } }));

import Navbar from './Navbar.svelte';

const currentUser: User = {
	id: 'u1',
	name: 'Alice',
	username: 'alice',
	avatar: null,
	bio: null,
	posts: 0,
	likes: 0,
	followers: 0,
	following: 0,
	isFollowing: false,
	created: new Date('2026-01-01T00:00:00Z')
};

let container: HTMLDivElement;
let instance: object | undefined;

afterEach(() => {
	if (instance) unmount(instance);
	container?.remove();
});

function render(props: ComponentProps<typeof Navbar>) {
	container = document.createElement('div');
	document.body.appendChild(container);
	instance = mount(Navbar, { target: container, props });
	flushSync();
	return container;
}

describe('Navbar notifications link', () => {
	it('disables SvelteKit preload-data so hovering the bell cannot trigger the mark-read load', () => {
		const el = render({ currentUser });

		const bellLink = el.querySelector('a[href="/notifications"]');
		expect(bellLink?.getAttribute('data-sveltekit-preload-data')).toBe('off');
	});
});
