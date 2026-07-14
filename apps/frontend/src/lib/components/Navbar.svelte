<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { Bell, Camera, Home, LogIn, Search, SquarePlus, User, UserPlus } from '@lucide/svelte';
	import type { User as UserType } from '$lib/types';

	let { currentUser, unreadCount = 0 }: { currentUser: UserType | null; unreadCount?: number } =
		$props();

	function isActive(path: string) {
		return page.url.pathname === path || page.url.pathname.startsWith(path + '/');
	}
</script>

<header
	class="sticky top-4 z-10 mx-auto flex h-16 w-full items-center justify-between gap-3 rounded-full border border-base-300 bg-base-100/75 px-3 py-2 text-base-content shadow-lg shadow-slate-900/10 backdrop-blur-xl sm:px-5"
>
	<a
		class="flex min-w-0 items-center gap-2 no-underline transition-transform duration-200 hover:scale-[1.01] active:scale-[0.99] sm:gap-3"
		href={resolve('/')}
		aria-label="Phasma home"
	>
		<span
			class="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-neutral text-neutral-content shadow-md shadow-slate-900/15"
		>
			<Camera class="h-5 w-5" />
		</span>
		<span class="hidden truncate text-2xl font-extrabold tracking-normal sm:inline sm:text-[1.5rem]"
			>Phasma</span
		>
	</a>

	<nav
		class="flex shrink-0 items-center gap-1 rounded-full bg-base-content/5 p-1 leading-none sm:gap-2"
		aria-label="Primary navigation"
	>
		{#if currentUser}
			<a
				href={resolve('/feed')}
				class="grid h-10 w-10 shrink-0 place-items-center rounded-full text-base-content/70 transition-colors hover:bg-base-content/10 hover:text-base-content"
				class:bg-white={isActive('/feed')}
				class:text-slate-950={isActive('/feed')}
				class:shadow-md={isActive('/feed')}
				title="Home"
				aria-label="Home"
			>
				<Home class="h-5 w-5" />
			</a>

			<a
				href={resolve('/search')}
				class="grid h-10 w-10 shrink-0 place-items-center rounded-full text-base-content/70 transition-colors hover:bg-base-content/10 hover:text-base-content"
				class:bg-white={isActive('/search')}
				class:text-slate-950={isActive('/search')}
				class:shadow-md={isActive('/search')}
				title="Search"
				aria-label="Search"
			>
				<Search class="h-5 w-5" />
			</a>

			<a
				href={resolve('/upload')}
				class="grid h-10 w-10 shrink-0 place-items-center rounded-full text-base-content/70 transition-colors hover:bg-base-content/10 hover:text-base-content"
				class:bg-white={isActive('/upload')}
				class:text-slate-950={isActive('/upload')}
				class:shadow-md={isActive('/upload')}
				title="Upload"
				aria-label="Upload"
			>
				<SquarePlus class="h-5 w-5" />
			</a>

			<a
				href={resolve('/notifications')}
				class="relative grid h-10 w-10 shrink-0 place-items-center rounded-full text-base-content/70 transition-colors hover:bg-base-content/10 hover:text-base-content"
				class:bg-white={isActive('/notifications')}
				class:text-slate-950={isActive('/notifications')}
				class:shadow-md={isActive('/notifications')}
				title="Notifications"
				aria-label="Notifications"
			>
				<Bell class="h-5 w-5" />
				{#if unreadCount > 0}
					<span
						class="absolute right-1.5 top-1.5 flex h-2 w-2 items-center justify-center rounded-full bg-primary"
						aria-label="{unreadCount} unread notification{unreadCount === 1 ? '' : 's'}"
					></span>
				{/if}
			</a>

			<a
				href={resolve(`/@${currentUser.username}`)}
				class="grid h-10 w-10 shrink-0 place-items-center rounded-full text-base-content/70 transition-colors hover:bg-base-content/10 hover:text-base-content"
				class:bg-white={isActive('/@' + currentUser.username)}
				class:text-slate-950={isActive('/@' + currentUser.username)}
				class:shadow-md={isActive('/@' + currentUser.username)}
				title="Profile"
				aria-label="Profile"
			>
				<User class="h-5 w-5" />
			</a>
		{:else}
			<a
				href={resolve('/login')}
				class="flex h-10 shrink-0 items-center gap-1.5 rounded-full px-4 text-sm font-bold text-base-content/70 transition-colors hover:bg-base-content/10 hover:text-base-content"
				class:bg-white={isActive('/login')}
				class:text-slate-950={isActive('/login')}
				class:shadow-md={isActive('/login')}
			>
				<LogIn class="h-5 w-5" />
				<span class="hidden sm:inline">Log In</span>
			</a>

			<a
				href={resolve('/register')}
				class="flex h-10 shrink-0 items-center gap-1.5 rounded-full px-4 text-sm font-bold text-base-content/70 transition-colors hover:bg-base-content/10 hover:text-base-content"
				class:bg-white={isActive('/register')}
				class:text-slate-950={isActive('/register')}
				class:shadow-md={isActive('/register')}
			>
				<UserPlus class="h-5 w-5" />
				<span class="hidden sm:inline">Register</span>
			</a>
		{/if}
	</nav>
</header>
