import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ parent }) => {
	const { currentUser } = await parent();
	if (!currentUser) throw redirect(303, '/login');
};
