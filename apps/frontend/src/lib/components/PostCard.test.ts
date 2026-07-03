import { describe, it, expect, afterEach } from 'vitest';
import { mount, unmount, flushSync, type ComponentProps } from 'svelte';
import PostCard from './PostCard.svelte';
import type { Post } from '$lib/types';

const post: Post = {
	publicId: 'post-1',
	username: 'alice',
	name: 'Alice',
	avatar: null,
	filename: 'photo.jpg',
	description: 'hello',
	created: new Date('2026-01-01T00:00:00Z'),
	likes: 3,
	liked: false,
	comments: 1
};

let container: HTMLDivElement;
let instance: object | undefined;

afterEach(() => {
	if (instance) unmount(instance);
	container?.remove();
});

function renderPostCard(props: ComponentProps<typeof PostCard>) {
	container = document.createElement('div');
	document.body.appendChild(container);
	instance = mount(PostCard, { target: container, props });
	flushSync();
	return container;
}

describe('PostCard anonymous gating', () => {
	it('renders a login link instead of the like form when currentUsername is null', () => {
		const el = renderPostCard({ post, currentUsername: null });
		expect(el.querySelector('a[href="/login"]')).not.toBeNull();
		expect(el.querySelector('form[action*="?/like"]')).toBeNull();
	});

	it('renders a login link instead of the comment form in singleView when currentUsername is null', () => {
		const el = renderPostCard({ post, currentUsername: null, singleView: true, comments: [] });
		const loginLinks = el.querySelectorAll('a[href="/login"]');
		expect(loginLinks.length).toBeGreaterThan(0);
		expect(Array.from(loginLinks).some((a) => a.textContent?.includes('Log in to comment'))).toBe(
			true
		);
		expect(el.querySelector('form[action*="?/comment"]')).toBeNull();
	});
});
