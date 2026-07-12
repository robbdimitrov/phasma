import { describe, it, expect } from 'vitest';
import { mapUser, mapPost, mapComment, mapSession, mapNotification } from '$lib/utils/mappers';
import type { UserDto, PostDto, CommentDto, SessionDto, NotificationDto } from '$lib/types';

const baseUserDto: UserDto = {
	id: '550e8400-e29b-41d4-a716-446655440001',
	name: 'John Doe',
	username: 'johndoe',
	email: 'john@example.com',
	avatar: 'avatar.jpg',
	bio: 'Hello world',
	posts: 10,
	likes: 50,
	followers: 100,
	following: 20,
	isFollowing: false,
	created: '2024-01-01T00:00:00Z'
};

const basePostDto: PostDto = {
	publicId: 'abc123',
	username: 'johndoe',
	name: 'John Doe',
	avatar: 'avatar.jpg',
	filename: 'photo.jpg',
	description: 'A photo',
	likes: 15,
	liked: false,
	comments: 3,
	created: '2024-01-01T00:00:00Z'
};

const baseCommentDto: CommentDto = {
	id: '550e8400-e29b-41d4-a716-446655440000',
	username: 'johndoe',
	avatar: null,
	body: 'Nice photo!',
	created: '2024-01-02T00:00:00Z'
};

describe('mapUser', () => {
	it('maps DTO to User with Date object', () => {
		const user = mapUser(baseUserDto);
		expect(user.id).toBe('550e8400-e29b-41d4-a716-446655440001');
		expect(user.name).toBe('John Doe');
		expect(user.created).toBeInstanceOf(Date);
	});

	it('defaults followers/following/isFollowing when missing', () => {
		const dto = {
			...baseUserDto,
			followers: undefined as unknown as number,
			following: undefined as unknown as number,
			isFollowing: undefined as unknown as boolean
		};
		const user = mapUser(dto);
		expect(user.followers).toBe(0);
		expect(user.following).toBe(0);
		expect(user.isFollowing).toBe(false);
	});
});

describe('mapPost', () => {
	it('maps DTO to Post with Date object', () => {
		const post = mapPost(basePostDto);
		expect(post.publicId).toBe('abc123');
		expect(post.created).toBeInstanceOf(Date);
	});

	it('defaults comments count when missing', () => {
		const dto = { ...basePostDto, comments: undefined as unknown as number };
		const post = mapPost(dto);
		expect(post.comments).toBe(0);
	});
});

describe('mapComment', () => {
	it('maps DTO to Comment with Date object', () => {
		const comment = mapComment(baseCommentDto);
		expect(comment.id).toBe('550e8400-e29b-41d4-a716-446655440000');
		expect(comment.body).toBe('Nice photo!');
		expect(comment.created).toBeInstanceOf(Date);
	});

	it('preserves null avatar', () => {
		const comment = mapComment(baseCommentDto);
		expect(comment.avatar).toBeNull();
	});
});

describe('mapSession', () => {
	const session: SessionDto = {
		id: '01904d2e-7f4d-7c33-ae21-2f94737eaa10',
		created: '2026-06-22T12:00:00Z',
		expiresAt: '2026-06-29T12:00:00Z',
		current: true
	};

	it('maps both timestamps to valid Date values', () => {
		expect(mapSession(session)).toEqual({
			...session,
			created: new Date(session.created),
			expiresAt: new Date(session.expiresAt)
		});
	});

	it.each(['created', 'expiresAt'] as const)('rejects an invalid %s timestamp', (field) => {
		expect(() => mapSession({ ...session, [field]: 'invalid' })).toThrow('Invalid timestamp');
	});
});

describe('mapNotification', () => {
	const notification: NotificationDto = {
		id: '01904d2e-7f4d-7c33-ae21-2f94737eaa10',
		actorUsername: 'johndoe',
		actorName: 'John Doe',
		actorAvatar: 'avatar.jpg',
		type: 'like',
		entityId: '01904d2e-7f4d-7c33-ae21-2f94737eab20',
		read: false,
		created: '2026-06-22T12:00:00Z'
	};

	it('maps DTO to Notification with Date object and actor fields', () => {
		const mapped = mapNotification(notification);
		expect(mapped).toEqual({
			...notification,
			created: new Date(notification.created)
		});
	});

	it('preserves a null actor avatar', () => {
		const mapped = mapNotification({ ...notification, actorAvatar: null });
		expect(mapped.actorAvatar).toBeNull();
	});
});
