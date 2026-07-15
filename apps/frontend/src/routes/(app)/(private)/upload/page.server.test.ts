import { error } from '@sveltejs/kit';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, uploadImage, createPost } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	uploadImage: vi.fn(),
	createPost: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/posts', () => ({ uploadImage, createPost }));

import { actions } from './+page.server';

function defaultAction() {
	const action = actions.default;
	if (!action) throw new Error('default action is not defined');
	return action;
}

function actionEvent({
	session = 'session-token',
	file
}: {
	session?: string | null;
	file?: File;
} = {}): Parameters<ReturnType<typeof defaultAction>>[0] {
	const formData = {
		get: (key: string) =>
			key === 'image' ? (file ?? null) : key === 'description' ? 'hello' : null
	};
	return {
		cookies: { get: vi.fn(() => session ?? undefined) },
		fetch: vi.fn(),
		request: { formData: vi.fn().mockResolvedValue(formData) }
	} as unknown as Parameters<ReturnType<typeof defaultAction>>[0];
}

beforeEach(() => {
	vi.clearAllMocks();
});

function backendError(status: number): unknown {
	try {
		error(status, 'backend details');
	} catch (cause) {
		return cause;
	}
}

describe('upload action', () => {
	it('redirects anonymous submissions before validating upload input', async () => {
		await expect(defaultAction()(actionEvent({ session: null }))).rejects.toMatchObject({
			status: 303,
			location: '/login'
		});
		expect(uploadImage).not.toHaveBeenCalled();
		expect(createPost).not.toHaveBeenCalled();
	});

	it('redirects expired sessions to /login', async () => {
		const file = new File(['image'], 'photo.jpg', { type: 'image/jpeg' });
		uploadImage.mockRejectedValue(backendError(401));

		await expect(defaultAction()(actionEvent({ file }))).rejects.toMatchObject({
			status: 303,
			location: '/login'
		});
		expect(createPost).not.toHaveBeenCalled();
	});
});
