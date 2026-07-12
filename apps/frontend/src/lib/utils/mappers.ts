import type {
	User,
	UserDto,
	Post,
	PostDto,
	Comment,
	CommentDto,
	Session,
	SessionDto,
	Notification,
	NotificationDto
} from '$lib/types';

export function mapUser(dto: UserDto): User {
	return {
		id: dto.id,
		name: dto.name,
		username: dto.username,
		email: dto.email,
		avatar: dto.avatar,
		bio: dto.bio,
		posts: dto.posts,
		likes: dto.likes,
		followers: dto.followers ?? 0,
		following: dto.following ?? 0,
		isFollowing: dto.isFollowing ?? false,
		created: new Date(dto.created)
	};
}

export function mapPost(dto: PostDto): Post {
	return {
		publicId: dto.publicId,
		username: dto.username,
		name: dto.name,
		avatar: dto.avatar,
		filename: dto.filename,
		description: dto.description,
		likes: dto.likes,
		liked: dto.liked,
		comments: dto.comments ?? 0,
		created: new Date(dto.created)
	};
}

export function mapComment(dto: CommentDto): Comment {
	return {
		id: dto.id,
		username: dto.username,
		avatar: dto.avatar,
		body: dto.body,
		created: new Date(dto.created)
	};
}

export function mapSession(dto: SessionDto): Session {
	return {
		id: dto.id,
		created: parseTimestamp(dto.created),
		expiresAt: parseTimestamp(dto.expiresAt),
		current: dto.current
	};
}

export function mapNotification(dto: NotificationDto): Notification {
	return {
		id: dto.id,
		actorUsername: dto.actorUsername,
		actorName: dto.actorName,
		actorAvatar: dto.actorAvatar,
		type: dto.type,
		entityId: dto.entityId,
		read: dto.read,
		created: new Date(dto.created)
	};
}

function parseTimestamp(value: string): Date {
	const timestamp = new Date(value);
	if (!Number.isFinite(timestamp.getTime())) {
		throw new TypeError('Invalid timestamp');
	}
	return timestamp;
}
