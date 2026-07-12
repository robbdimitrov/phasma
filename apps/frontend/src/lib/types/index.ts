export interface CursorPage<T> {
	items: T[];
	nextCursor: string | null;
}

export interface UserDto {
	id: string;
	name: string;
	username: string;
	email: string;
	avatar: string | null;
	bio: string | null;
	posts: number;
	likes: number;
	followers: number;
	following: number;
	isFollowing: boolean;
	created: string;
}

export interface User {
	id: string;
	name: string;
	username: string;
	email?: string;
	avatar: string | null;
	bio: string | null;
	posts: number;
	likes: number;
	followers: number;
	following: number;
	isFollowing: boolean;
	created: Date;
}

export interface PostIdDto {
	publicId: string;
}

export interface PostDto {
	publicId: string;
	username: string;
	name: string;
	avatar: string | null;
	filename: string;
	description: string | null;
	likes: number;
	liked: boolean;
	comments: number;
	created: string;
}

export interface ImageFilenameDto {
	filename: string;
}

export interface Post {
	publicId: string;
	username: string;
	name: string;
	avatar: string | null;
	filename: string;
	description: string | null;
	likes: number;
	liked: boolean;
	comments: number;
	created: Date;
}

export interface CommentDto {
	id: string;
	username: string;
	avatar: string | null;
	body: string;
	created: string;
}

export interface Comment {
	id: string;
	username: string;
	avatar: string | null;
	body: string;
	created: Date;
}

export interface SessionDto {
	id: string;
	created: string;
	expiresAt: string;
	current: boolean;
}

export interface Session {
	id: string;
	created: Date;
	expiresAt: Date;
	current: boolean;
}

export type NotificationType = 'like' | 'comment' | 'follow';

export interface NotificationDto {
	id: string;
	actorUsername: string;
	actorName: string;
	actorAvatar: string | null;
	type: NotificationType;
	entityId: string;
	read: boolean;
	created: string;
}

export interface Notification {
	id: string;
	actorUsername: string;
	actorName: string;
	actorAvatar: string | null;
	type: NotificationType;
	entityId: string;
	read: boolean;
	created: Date;
}
