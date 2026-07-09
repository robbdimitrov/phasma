<script lang="ts">
	import { resolve } from '$app/paths';
	import { enhance } from '$app/forms';
	import type { ActionData } from './$types';

	let { form }: { form: ActionData } = $props();

	let showPassword = $state(false);
	let submitting = $state(false);
</script>

<svelte:head>
	<title>Register — Phasma</title>
</svelte:head>

<section
	class="mx-auto grid w-full max-w-xl gap-6 rounded-2xl border border-base-300 bg-base-100/90 p-6 text-base-content shadow-2xl shadow-slate-900/10 backdrop-blur-xl dark:shadow-black/35 sm:px-8"
>
	<div class="grid gap-3">
		<span class="text-xs font-extrabold uppercase tracking-[0.16em] text-base-content/60"
			>Join Phasma</span
		>
		<h1 class="text-3xl font-extrabold leading-tight tracking-normal text-base-content sm:text-4xl">
			Create your account
		</h1>
		<p class="text-base leading-7 text-base-content/70">
			Start a photo journal, publish your best moments, and make your profile yours.
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
			<label for="name" class="label p-0">
				<span
					class="label-text text-xs font-extrabold uppercase tracking-widest text-base-content/80"
					>Full Name</span
				>
			</label>
			<input
				type="text"
				id="name"
				name="name"
				placeholder="John Doe"
				maxlength="100"
				autocomplete="name"
				required
				class="input input-bordered w-full rounded-xl"
			/>
		</div>

		<div class="form-control grid gap-2">
			<label for="username" class="label p-0">
				<span
					class="label-text text-xs font-extrabold uppercase tracking-widest text-base-content/80"
					>Username</span
				>
			</label>
			<input
				type="text"
				id="username"
				name="username"
				placeholder="johndoe"
				autocomplete="username"
				pattern="[a-z0-9._]&#123;3,30&#125;"
				minlength="3"
				maxlength="30"
				required
				class="input input-bordered w-full rounded-xl"
			/>
		</div>

		<div class="form-control grid gap-2">
			<label for="email" class="label p-0">
				<span
					class="label-text text-xs font-extrabold uppercase tracking-widest text-base-content/80"
					>Email Address</span
				>
			</label>
			<input
				type="email"
				id="email"
				name="email"
				placeholder="johndoe@mail.com"
				pattern="[^@]+@[^@]+\.[^@]+"
				autocomplete="email"
				required
				class="input input-bordered w-full rounded-xl"
			/>
		</div>

		<div class="form-control grid gap-2">
			<label for="password" class="label p-0">
				<span
					class="label-text text-xs font-extrabold uppercase tracking-widest text-base-content/80"
					>Password</span
				>
			</label>
			<div class="relative flex items-center">
				<input
					type={showPassword ? 'text' : 'password'}
					id="password"
					name="password"
					placeholder="Password"
					minlength="8"
					maxlength="30"
					autocomplete="new-password"
					required
					class="input input-bordered w-full rounded-xl pr-20"
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
			{submitting ? 'Creating Account...' : 'Create Account'}
		</button>
	</form>

	<div class="grid gap-3 border-t border-base-300 pt-6 text-center">
		<span class="text-xs font-extrabold uppercase tracking-[0.16em] text-base-content/60"
			>Already registered?</span
		>
		<p class="text-sm text-base-content/70">
			Already have an account?
			<a href={resolve('/login')} class="ml-1 font-extrabold text-base-content hover:text-primary"
				>Log in</a
			>
		</p>
	</div>
</section>
