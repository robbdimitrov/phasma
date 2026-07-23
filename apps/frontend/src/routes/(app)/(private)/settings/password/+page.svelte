<script lang="ts">
	import { resolve } from '$app/paths';
	import { enhance } from '$app/forms';
	import { ArrowLeft } from '@lucide/svelte';
	import { pageTitle } from '$lib/pageTitle';
	import type { ActionData } from './$types';

	let { form }: { form: ActionData } = $props();

	function initialFormError() {
		return form?.error ?? '';
	}

	let showOldPassword = $state(false);
	let showNewPassword = $state(false);
	let submitting = $state(false);
	let errorMessage = $state(initialFormError());
</script>

<svelte:head>
	<title>{pageTitle('Change Password')}</title>
</svelte:head>

<div
	class="rounded-2xl border border-base-300 bg-base-100 text-base-content mx-auto flex max-w-xl flex-col gap-6 p-6 shadow-lg shadow-slate-900/5 sm:px-8"
>
	<div class="flex items-center gap-3 border-b border-base-300 pb-4">
		<a
			href={resolve('/settings')}
			class="btn btn-ghost btn-circle btn-sm text-base-content transition-colors hover:bg-base-200"
			title="Back to Settings"
		>
			<ArrowLeft class="h-5 w-5" />
		</a>
		<h1 class="text-2xl font-black text-base-content">Change Password</h1>
	</div>

	<form
		method="POST"
		class="grid gap-6"
		use:enhance={() => {
			submitting = true;
			errorMessage = '';
			return async ({ result, update }) => {
				submitting = false;
				if (result.type === 'failure') {
					errorMessage = (result.data as { error?: string })?.error ?? 'Could not update password.';
				}
				await update();
			};
		}}
	>
		{#if errorMessage || form?.error}
			<div
				class="rounded-2xl border border-error/30 bg-error/10 px-4 py-3 text-sm font-semibold text-error"
			>
				{errorMessage || form?.error}
			</div>
		{/if}

		<div class="form-control grid gap-2">
			<label for="old-password" class="label p-0">
				<span class="label-text text-xs font-bold uppercase tracking-wider text-base-content/80"
					>Current Password</span
				>
			</label>
			<div class="relative flex items-center">
				<input
					type={showOldPassword ? 'text' : 'password'}
					id="old-password"
					name="oldPassword"
					placeholder="Current password"
					maxlength="128"
					autocomplete="current-password"
					required
					class="input w-full rounded-xl pr-20"
				/>
				<button
					type="button"
					class="btn btn-ghost absolute right-1.5 h-10 min-h-10 rounded-full px-4 text-sm font-extrabold"
					onclick={() => (showOldPassword = !showOldPassword)}
				>
					{showOldPassword ? 'Hide' : 'Show'}
				</button>
			</div>
		</div>

		<div class="form-control grid gap-2">
			<label for="password" class="label p-0">
				<span class="label-text text-xs font-bold uppercase tracking-wider text-base-content/80"
					>New Password</span
				>
			</label>
			<div class="relative flex items-center">
				<input
					type={showNewPassword ? 'text' : 'password'}
					id="password"
					name="password"
					placeholder="New password"
					minlength="8"
					maxlength="128"
					autocomplete="new-password"
					required
					class="input w-full rounded-xl pr-20"
				/>
				<button
					type="button"
					class="btn btn-ghost absolute right-1.5 h-10 min-h-10 rounded-full px-4 text-sm font-extrabold"
					onclick={() => (showNewPassword = !showNewPassword)}
				>
					{showNewPassword ? 'Hide' : 'Show'}
				</button>
			</div>
		</div>

		<button
			type="submit"
			disabled={submitting}
			class="btn btn-neutral h-12 min-h-12 w-full rounded-full text-base font-extrabold shadow-lg shadow-slate-900/15"
		>
			{submitting ? 'Updating Password...' : 'Update Password'}
		</button>
	</form>
</div>
