import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { getCurrent, updateUser } from '$lib/server/api/users';
import { uploadImage } from '$lib/server/api/posts';
import { apiClient } from '$lib/server/api/client';
import { validateImageUpload } from '$lib/server/imageUpload';

export const load: PageServerLoad = async ({ fetch, cookies, parent }) => {
	const [parentData, currentUserWithEmail] = await Promise.all([
		parent(),
		getCurrent(apiClient({ fetch, cookies }))
	]);
	if (!currentUserWithEmail) throw redirect(303, '/login');
	return { ...parentData, currentUser: currentUserWithEmail };
};

export const actions: Actions = {
	default: async ({ fetch, cookies, request }) => {
		const api = apiClient({ fetch, cookies });
		const currentUser = await getCurrent(api);
		if (!currentUser) throw redirect(303, '/login');

		const data = await request.formData();
		const name = ((data.get('name') as string) ?? '').trim();
		const username = ((data.get('username') as string) ?? '').trim().toLowerCase();
		const email = ((data.get('email') as string) ?? '').trim();
		const bio = ((data.get('bio') as string) ?? '').trim();
		const removeAvatar = data.get('removeAvatar') === 'true';
		const file = data.get('avatar');

		if (!name || !username || !email) {
			return fail(400, { error: 'Name, username, and email are required.' });
		}

		let avatarFilename = currentUser.avatar ?? '';

		try {
			if (removeAvatar) {
				avatarFilename = '';
			} else if (file instanceof File && file.size > 0) {
				const image = validateImageUpload(file, 'Please select an image.');
				if (!image.ok) {
					return fail(image.status, { error: image.error });
				}
				const uploaded = await uploadImage(api, image.file);
				if (uploaded) avatarFilename = uploaded.filename;
			}

			await updateUser(api, name, username, email, avatarFilename, bio);
		} catch {
			return fail(400, {
				error: 'Could not save profile. Please check your details and try again.'
			});
		}

		throw redirect(303, '/settings');
	}
};
