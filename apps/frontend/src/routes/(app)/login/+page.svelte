<script lang="ts">
	import { resolve } from '$app/paths';
	import { enhance } from '$app/forms';
	import { pageTitle } from '$lib/pageTitle';
	import type { ActionData } from './$types';

	let { form }: { form: ActionData } = $props();

	let showPassword = $state(false);
	let submitting = $state(false);
</script>

<svelte:head>
	<title>{pageTitle('Log in')}</title>
</svelte:head>

<section
	class="mx-auto grid w-full max-w-xl gap-6 rounded-2xl border border-base-300 bg-base-100/90 p-6 text-base-content shadow-2xl shadow-slate-900/10 backdrop-blur-xl sm:px-8"
>
	<div class="grid gap-3">
		<span class="text-xs font-extrabold uppercase text-base-content/60">Welcome back</span>
		<h1 class="text-3xl font-extrabold leading-tight text-base-content sm:text-4xl">
			Log in to Phasma
		</h1>
		<p class="text-base leading-7 text-base-content/70">
			Share photos, follow the feed, and keep your profile moving.
		</p>
	</div>

	<form
		method="POST"
		class="grid gap-6"
		use:enhance={() => {
			submitting = true;
			return async ({ update }) => {
				submitting = false;
				await update();
			};
		}}
	>
		{#if form?.error}
			<div
				class="rounded-2xl border border-error/30 bg-error/10 px-4 py-3 text-sm font-semibold text-error"
			>
				{form.error}
			</div>
		{/if}

		<div class="form-control grid gap-2">
			<label for="email" class="label p-0">
				<span class="label-text text-xs font-bold uppercase tracking-wider text-base-content/80"
					>Email Address</span
				>
			</label>
			<input
				type="email"
				id="email"
				name="email"
				placeholder="johndoe@mail.com"
				autocomplete="username"
				required
				class="input w-full rounded-xl"
			/>
		</div>

		<div class="form-control grid gap-2">
			<label for="password" class="label p-0">
				<span class="label-text text-xs font-bold uppercase tracking-wider text-base-content/80"
					>Password</span
				>
			</label>
			<div class="relative flex items-center">
				<input
					type={showPassword ? 'text' : 'password'}
					id="password"
					name="password"
					placeholder="Password"
					maxlength="128"
					autocomplete="current-password"
					required
					class="input w-full rounded-xl pr-20"
				/>
				<button
					type="button"
					class="btn btn-ghost absolute right-1.5 h-10 min-h-10 rounded-full px-4 text-sm font-extrabold"
					onclick={() => (showPassword = !showPassword)}
				>
					{showPassword ? 'Hide' : 'Show'}
				</button>
			</div>
		</div>

		<button
			type="submit"
			disabled={submitting}
			class="btn btn-neutral h-12 min-h-12 w-full rounded-full text-base font-extrabold shadow-lg shadow-slate-900/15"
		>
			{submitting ? 'Logging in...' : 'Log in'}
		</button>
	</form>

	<div class="grid gap-3 border-t border-base-300 pt-6 text-center">
		<p class="text-sm text-base-content/70">
			Don't have an account?
			<a
				href={resolve('/register')}
				class="ml-1 font-extrabold text-base-content hover:text-primary">Create one</a
			>
		</p>
	</div>
</section>
