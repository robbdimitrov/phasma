<script lang="ts">
	import { resolve } from '$app/paths';
	import { enhance } from '$app/forms';
	import { ArrowLeft, Camera, Trash2 } from '@lucide/svelte';
	import { resizeImageForUpload } from '$lib/utils/image-resizer';
	import { imageUrl } from '$lib/utils/imageUrl';
	import type { PageData, ActionData } from './$types';

	let { data, form }: { data: PageData; form: ActionData } = $props();

	function initialState() {
		const user = data.currentUser;
		return {
			name: user.name,
			username: user.username,
			email: user.email,
			bio: user.bio ?? '',
			error: form?.error ?? ''
		};
	}

	const initialValues = initialState();
	const user = $derived(data.currentUser);
	let name = $state(initialValues.name);
	let username = $state(initialValues.username);
	let email = $state(initialValues.email);
	let bio = $state(initialValues.bio);
	let avatarPreview = $state('');
	let removeAvatar = $state(false);
	let selectedFile = $state<File | null>(null);
	let fileInput = $state<HTMLInputElement | null>(null);
	let errorMessage = $state(initialValues.error);
	let isSubmitting = $state(false);

	async function handleFileChange(files: FileList | null) {
		if (!files || files.length === 0) return;
		const file = files[0];
		if (!file) return;
		errorMessage = '';
		try {
			const resized = await resizeImageForUpload(file);
			selectedFile = resized;
			removeAvatar = false;
			const reader = new FileReader();
			reader.readAsDataURL(resized);
			reader.onload = () => {
				avatarPreview = reader.result as string;
			};
		} catch (e) {
			errorMessage = e instanceof Error ? e.message : 'Could not prepare avatar.';
		}
	}

	function handleRemoveAvatar() {
		selectedFile = null;
		avatarPreview = '';
		removeAvatar = true;
	}

	const currentAvatar = $derived(
		avatarPreview || (!removeAvatar ? imageUrl(user.avatar) : '/assets/placeholder.svg')
	);
</script>

<div
	class="rounded-2xl border border-base-300 bg-base-100 text-base-content mx-auto flex max-w-xl flex-col gap-6 p-6 shadow-lg shadow-slate-900/5 sm:px-8"
>
	<div class="flex items-center gap-3 border-b border-base-300 pb-4">
		<a
			href={resolve('/settings')}
			class="btn btn-ghost btn-circle btn-sm text-base-content/60 transition-colors hover:bg-base-200 hover:text-base-content"
			title="Back to Settings"
		>
			<ArrowLeft class="h-5 w-5" />
		</a>
		<h1 class="text-2xl font-black text-base-content">Edit Profile</h1>
	</div>

	{#if errorMessage}
		<div
			class="rounded-2xl border border-error/30 bg-error/10 px-4 py-3 text-sm font-semibold text-error"
		>
			{errorMessage}
		</div>
	{/if}

	<form
		method="POST"
		enctype="multipart/form-data"
		class="grid gap-6"
		use:enhance={({ formData }) => {
			isSubmitting = true;
			errorMessage = '';
			formData.delete('avatar');
			if (selectedFile) formData.set('avatar', selectedFile, selectedFile.name);
			formData.set('removeAvatar', String(removeAvatar));
			return async ({ result, update }) => {
				isSubmitting = false;
				if (result.type === 'failure') {
					errorMessage = (result.data as { error?: string })?.error ?? 'Could not save profile.';
				}
				await update();
			};
		}}
	>
		<div class="flex flex-col items-center gap-3">
			<div class="relative">
				<label
					for="avatar-input"
					class="group relative block cursor-pointer overflow-hidden rounded-full border border-base-300 shadow-md h-24 w-24 sm:h-28 sm:w-28 transition-transform hover:scale-[1.02]"
					title="Change Avatar"
				>
					<img class="h-full w-full object-cover" src={currentAvatar} alt="Profile avatar" />
					<div
						class="absolute inset-0 flex flex-col items-center justify-center bg-slate-950/40 text-white opacity-0 backdrop-blur-[2px] transition-all duration-300 group-hover:opacity-100"
					>
						<Camera class="h-6 w-6" />
						<span class="mt-1 text-[10px] font-bold uppercase tracking-wider">Change</span>
					</div>
				</label>

				{#if avatarPreview || (user.avatar && !removeAvatar)}
					<div class="absolute bottom-0 right-0 rounded-full bg-base-100 shadow-sm">
						<button
							type="button"
							class="inline-flex h-9 w-9 items-center justify-center rounded-full border border-base-300 text-base-content/50 transition-all duration-150 hover:border-rose-200 hover:bg-rose-50 hover:text-rose-600 active:scale-95 dark:hover:border-rose-900/50 dark:hover:bg-rose-500/10 dark:hover:text-rose-400"
							title="Remove avatar"
							onclick={handleRemoveAvatar}
						>
							<Trash2 class="h-4 w-4" />
						</button>
					</div>
				{/if}
			</div>
			<span class="text-xs font-medium text-base-content/60">Tap image to change</span>
		</div>

		<!-- Hidden file input (not submitted — we inject manually via enhance) -->
		<input
			bind:this={fileInput}
			type="file"
			id="avatar-input"
			name="avatar"
			accept="image/jpeg,image/png,image/gif,image/webp"
			class="hidden"
			onchange={(e) => handleFileChange(e.currentTarget.files)}
		/>

		<input type="hidden" name="removeAvatar" value={String(removeAvatar)} />

		<div class="grid gap-4">
			<div class="form-control grid gap-2">
				<label for="name" class="label p-0">
					<span class="label-text text-xs font-bold uppercase tracking-wider text-base-content/80"
						>Full Name</span
					>
				</label>
				<input
					type="text"
					id="name"
					name="name"
					bind:value={name}
					placeholder="Enter your name"
					autocomplete="name"
					required
					class="input input-bordered w-full rounded-xl"
				/>
			</div>

			<div class="form-control grid gap-2">
				<label for="username" class="label p-0">
					<span class="label-text text-xs font-bold uppercase tracking-wider text-base-content/80"
						>Username</span
					>
				</label>
				<input
					type="text"
					id="username"
					name="username"
					bind:value={username}
					placeholder="Enter username"
					autocomplete="username"
					minlength="3"
					maxlength="30"
					required
					class="input input-bordered w-full rounded-xl"
				/>
			</div>

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
					bind:value={email}
					placeholder="Enter email"
					autocomplete="email"
					required
					class="input input-bordered w-full rounded-xl"
				/>
			</div>

			<div class="form-control grid gap-2">
				<label for="bio" class="label p-0">
					<span class="label-text text-xs font-bold uppercase tracking-wider text-base-content/80"
						>Bio</span
					>
				</label>
				<textarea
					id="bio"
					name="bio"
					bind:value={bio}
					placeholder="Tell people about yourself..."
					maxlength="300"
					autocomplete="off"
					class="textarea textarea-bordered h-24 w-full resize-none rounded-xl"></textarea>
			</div>
		</div>

		<button
			type="submit"
			disabled={isSubmitting}
			class="btn btn-neutral h-12 min-h-12 w-full rounded-full text-base font-extrabold shadow-lg shadow-slate-900/15"
		>
			{isSubmitting ? 'Saving Profile...' : 'Save Profile Details'}
		</button>
	</form>
</div>
