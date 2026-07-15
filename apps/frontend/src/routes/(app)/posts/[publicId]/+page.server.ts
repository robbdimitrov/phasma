import { error, fail, isHttpError, redirect } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import {
	getPost,
	getComments,
	likePost,
	unlikePost,
	createComment,
	deleteComment,
	deletePost
} from '$lib/server/api/posts';
import { apiClient } from '$lib/server/api/client';

export const load: PageServerLoad = async ({ fetch, cookies, params, parent }) => {
	const api = apiClient({ fetch, cookies });
	const [{ currentUser }, post] = await Promise.all([parent(), getPost(api, params.publicId)]);
	if (!post) throw error(404, 'Post not found');
	if (!currentUser) {
		return {
			post,
			comments: [],
			nextCommentsCursor: null
		};
	}
	const commentsPage = await getComments(api, params.publicId);
	return {
		post,
		comments: commentsPage.items,
		nextCommentsCursor: commentsPage.nextCursor
	};
};

async function requireActionSession<T>(promise: Promise<T>): Promise<T> {
	try {
		return await promise;
	} catch (cause) {
		if (isHttpError(cause) && cause.status === 401) throw redirect(303, '/login');
		throw cause;
	}
}

export const actions: Actions = {
	like: async ({ fetch, cookies, params }) => {
		if (!cookies.get('session')) throw redirect(303, '/login');
		await requireActionSession(likePost(apiClient({ fetch, cookies }), params.publicId));
		return { success: true };
	},
	unlike: async ({ fetch, cookies, params }) => {
		if (!cookies.get('session')) throw redirect(303, '/login');
		await requireActionSession(unlikePost(apiClient({ fetch, cookies }), params.publicId));
		return { success: true };
	},
	comment: async ({ fetch, cookies, request, params }) => {
		if (!cookies.get('session')) throw redirect(303, '/login');
		const data = await request.formData();
		const body = ((data.get('body') as string) ?? '').trim();
		if (!body || body.length > 400)
			return fail(400, { error: 'Comment must be 1–400 characters.' });
		const comment = await requireActionSession(
			createComment(apiClient({ fetch, cookies }), params.publicId, body)
		);
		return { success: true, comment };
	},
	deleteComment: async ({ fetch, cookies, request, params }) => {
		if (!cookies.get('session')) throw redirect(303, '/login');
		const data = await request.formData();
		const commentId = (data.get('commentId') as string) ?? '';
		if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(commentId))
			return fail(400, { error: 'Invalid comment ID.' });
		await requireActionSession(
			deleteComment(apiClient({ fetch, cookies }), params.publicId, commentId)
		);
		return { success: true };
	},
	deletePost: async ({ fetch, cookies, params }) => {
		if (!cookies.get('session')) throw redirect(303, '/login');
		await requireActionSession(deletePost(apiClient({ fetch, cookies }), params.publicId));
		throw redirect(303, '/feed');
	}
};
