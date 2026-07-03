<script lang="ts">
	import { resolve } from '$app/paths';
	import { enhance } from '$app/forms';
	import { ChevronRight } from '@lucide/svelte';
	import { getThemeContext } from '$lib/theme.svelte';
	import type { ThemeMode } from '$lib/theme';

	const theme = getThemeContext();
	const options: ThemeMode[] = ['light', 'dark', 'system'];

	function optionTitle(opt: ThemeMode) {
		return opt.charAt(0).toUpperCase() + opt.slice(1);
	}

	function isActive(opt: ThemeMode) {
		return theme.mode === opt;
	}
</script>

<svelte:head>
	<title>Settings — Phasma</title>
</svelte:head>

<div class="mx-auto max-w-xl">
	<div
		class="rounded-2xl border border-base-300 bg-base-100 text-base-content flex w-full flex-col gap-6 p-6 shadow-lg shadow-slate-900/5 sm:px-8"
	>
		<div class="grid gap-2 text-center sm:text-left">
			<h1 class="text-2xl font-black text-base-content">Settings</h1>
			<p class="text-xs text-base-content/70">
				Manage your account settings, security and activity.
			</p>
		</div>

		<ul class="flex flex-col gap-3">
			<li class="rounded-2xl border border-base-300 bg-base-200 p-4">
				<div class="flex flex-col gap-3">
					<div class="grid gap-2">
						<h2 class="text-sm font-bold text-base-content">Appearance</h2>
						<p class="text-xs text-base-content/60">Choose how Phasma looks on this device.</p>
					</div>
					<div class="grid grid-cols-3 gap-2 rounded-full bg-base-100 p-1">
						{#each options as opt (opt)}
							<button
								type="button"
								class="h-10 rounded-full text-xs font-black transition-colors {isActive(opt)
									? 'bg-neutral text-neutral-content shadow-md'
									: 'text-base-content/60 hover:bg-base-200 hover:text-base-content'}"
								onclick={() => theme.set(opt)}
							>
								{optionTitle(opt)}
							</button>
						{/each}
					</div>
				</div>
			</li>

			<li>
				<a
					href={resolve('/settings/profile')}
					class="flex items-center justify-between rounded-2xl border border-base-300 bg-base-200 p-4 text-sm font-bold text-base-content transition-colors hover:border-base-content/20 hover:bg-base-300"
				>
					<span>Edit Profile</span>
					<ChevronRight class="h-4 w-4 text-base-content/60" />
				</a>
			</li>

			<li>
				<a
					href={resolve('/settings/password')}
					class="flex items-center justify-between rounded-2xl border border-base-300 bg-base-200 p-4 text-sm font-bold text-base-content transition-colors hover:border-base-content/20 hover:bg-base-300"
				>
					<span>Change Password</span>
					<ChevronRight class="h-4 w-4 text-base-content/60" />
				</a>
			</li>

			<li>
				<a
					href={resolve('/settings/sessions')}
					class="flex items-center justify-between rounded-2xl border border-base-300 bg-base-200 p-4 text-sm font-bold text-base-content transition-colors hover:border-base-content/20 hover:bg-base-300"
				>
					<span>Active Sessions</span>
					<ChevronRight class="h-4 w-4 text-base-content/60" />
				</a>
			</li>

			<li>
				<form method="POST" action="/logout" use:enhance>
					<button
						type="submit"
						class="btn btn-error btn-outline h-12 min-h-12 w-full rounded-full font-bold"
					>
						Logout Account
					</button>
				</form>
			</li>
		</ul>
	</div>
</div>
