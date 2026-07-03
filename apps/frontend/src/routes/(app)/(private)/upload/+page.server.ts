import { fail, redirect } from '@sveltejs/kit';
import type { Actions } from './$types';
import { uploadImage, createPost } from '$lib/server/api/posts';
import { apiClient } from '$lib/server/api/client';
import { validateImageUpload } from '$lib/server/imageUpload';

export const actions: Actions = {
	default: async ({ fetch, cookies, request }) => {
		const api = apiClient({ fetch, cookies });
		const data = await request.formData();
		const file = data.get('image');
		const description = ((data.get('description') as string) ?? '').trim();

		const image = validateImageUpload(file, 'Please select an image.');
		if (!image.ok) {
			return fail(image.status, { error: image.error });
		}

		let uploaded;
		try {
			uploaded = await uploadImage(api, image.file);
		} catch {
			return fail(500, { error: 'Image upload failed. Please try again.' });
		}
		if (!uploaded) {
			return fail(500, { error: 'Image upload failed. Please try again.' });
		}

		let post;
		try {
			post = await createPost(api, uploaded.filename, description);
		} catch {
			return fail(500, { error: 'Could not create post. Please try again.' });
		}
		if (!post) {
			return fail(500, { error: 'Could not create post. Please try again.' });
		}

		throw redirect(303, '/feed');
	}
};
