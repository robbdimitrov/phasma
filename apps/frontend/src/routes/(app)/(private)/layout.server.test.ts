import { isRedirect } from '@sveltejs/kit';
import { describe, expect, it, vi } from 'vitest';

import { load } from './+layout.server';

function loadEvent(parent: () => Promise<{ currentUser: unknown }>): Parameters<typeof load>[0] {
	return { parent } as unknown as Parameters<typeof load>[0];
}

describe('(private) layout guard', () => {
	it('redirects anonymous visitors to /login', async () => {
		const parent = vi.fn().mockResolvedValue({ currentUser: null });

		try {
			await load(loadEvent(parent));
			expect.unreachable('expected a redirect to be thrown');
		} catch (err) {
			if (!isRedirect(err)) throw err;
			expect(err.status).toBe(303);
			expect(err.location).toBe('/login');
		}
	});

	it('does not redirect a signed-in visitor', async () => {
		const parent = vi.fn().mockResolvedValue({ currentUser: { id: '1', username: 'alice' } });

		await expect(load(loadEvent(parent))).resolves.toBeUndefined();
	});
});
