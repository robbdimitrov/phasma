<script lang="ts">
	import { enhance } from '$app/forms';
	import { SquarePlus } from '@lucide/svelte';
	import { resizeImageForUpload, supportedUploadMimeTypes } from '$lib/utils/image-resizer';
	import type { ActionData } from './$types';

	const MAX_DESCRIPTION = 1000;

	let { form }: { form: ActionData } = $props();

	let imagePreview = $state('');
	let errorMessage = $derived(form?.error ?? '');
	let isSubmitting = $state(false);
	let isDragging = $state(false);
	let description = $state('');
	let selectedFile = $state<File | undefined>(undefined);
	let fileInput = $state<HTMLInputElement | null>(null);

	async function selectFile(file: File | null | undefined) {
		errorMessage = '';
		if (!file) return;
		try {
			const resized = await resizeImageForUpload(file);
			selectedFile = resized;
			const reader = new FileReader();
			reader.readAsDataURL(resized);
			reader.onload = () => {
				imagePreview = reader.result as string;
			};
		} catch (e) {
			selectedFile = undefined;
			imagePreview = '';
			errorMessage = e instanceof Error ? e.message : 'Could not prepare image.';
		}
	}

	function handleDragOver(e: DragEvent) {
		e.preventDefault();
		isDragging = true;
	}

	function handleDragLeave(e: DragEvent) {
		e.preventDefault();
		isDragging = false;
	}

	function handleDrop(e: DragEvent) {
		e.preventDefault();
		isDragging = false;
		selectFile(e.dataTransfer?.files.item(0));
	}
</script>

<svelte:head>
	<title>Create Post — Phasma</title>
</svelte:head>

<div class="mx-auto max-w-5xl">
	<div
		class="rounded-2xl border border-base-300 bg-base-100 text-base-content overflow-hidden shadow-xl shadow-slate-900/5"
	>
		<div class="flex min-h-16 items-center justify-between border-b border-base-300 px-6 sm:px-8">
			<div>
				<p class="text-xs font-bold uppercase tracking-wider text-base-content/60">Phasma</p>
				<h1 class="text-lg font-black leading-tight">Create new post</h1>
			</div>
			<button
				type="submit"
				form="upload-form"
				disabled={!selectedFile || isSubmitting}
				class="btn btn-neutral btn-sm min-h-10 rounded-full px-5 font-black shadow-lg shadow-slate-900/10"
			>
				{isSubmitting ? 'Sharing...' : 'Share'}
			</button>
		</div>

		<form
			id="upload-form"
			method="POST"
			enctype="multipart/form-data"
			use:enhance={({ formData }) => {
				isSubmitting = true;
				errorMessage = '';
				// Replace file input with the already-resized file
				formData.delete('image');
				if (selectedFile) formData.set('image', selectedFile, selectedFile.name);
				return async ({ result, update }) => {
					isSubmitting = false;
					if (result.type === 'failure') {
						errorMessage = (result.data as { error?: string })?.error ?? 'Could not share post.';
					}
					await update();
				};
			}}
		>
			<div class="grid min-h-136 md:grid-cols-[minmax(0,1fr)_22rem]">
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div
					class={[
						'flex min-h-96 items-center justify-center bg-base-200 p-6 transition-colors sm:px-8',
						isDragging && 'bg-base-300'
					]
						.filter(Boolean)
						.join(' ')}
					ondragover={handleDragOver}
					ondragleave={handleDragLeave}
					ondrop={handleDrop}
				>
					{#if imagePreview}
						<div class="flex w-full max-w-2xl flex-col gap-4">
							<div class="aspect-square w-full overflow-hidden rounded-xl bg-base-200 shadow-sm">
								<img class="h-full w-full object-cover" src={imagePreview} alt="Upload preview" />
							</div>
							<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
								<p class="text-sm text-base-content/60">
									This is how your photo will appear in the feed.
								</p>
								<button
									type="button"
									class="btn btn-sm btn-outline min-h-10 shrink-0 rounded-full px-5 font-black shadow-sm"
									onclick={() => fileInput?.click()}
								>
									Change photo
								</button>
							</div>
						</div>
					{:else}
						<label
							class="flex aspect-square w-full max-w-2xl cursor-pointer flex-col items-center justify-center gap-4 rounded-2xl border border-dashed border-base-300 p-6 text-center transition-colors hover:border-base-content/30 hover:bg-base-100/60 sm:px-8"
							for="file-input"
						>
							<span
								class="grid h-16 w-16 place-items-center rounded-full bg-neutral text-neutral-content shadow-md shadow-slate-900/15"
							>
								<SquarePlus class="h-7 w-7" />
							</span>
							<span class="text-2xl font-black leading-tight">Drop your photo here</span>
							<span class="max-w-xs text-sm leading-6 text-base-content/60"
								>Choose a JPEG, PNG, GIF, or WEBP. Large images are resized automatically.</span
							>
							<span class="btn btn-sm btn-neutral min-h-10 rounded-full px-5 font-bold"
								>Select from computer</span
							>
						</label>
					{/if}

					<input
						bind:this={fileInput}
						type="file"
						id="file-input"
						name="image"
						accept={supportedUploadMimeTypes.join(',')}
						class="hidden"
						onchange={(e) => selectFile(e.currentTarget.files?.item(0))}
					/>
				</div>

				<div
					class="flex flex-col border-t border-base-300 bg-base-200/60 md:border-l md:border-t-0"
				>
					<div
						class="flex items-center justify-between gap-3 border-b border-base-300 px-6 py-4 sm:px-8"
					>
						<div class="grid gap-2">
							<span class="text-sm font-black">Caption</span>
							<p class="text-xs text-base-content/60">Give your post a little context.</p>
						</div>
						<span class="rounded-full bg-base-300 px-3 py-1 text-xs font-bold text-base-content/60"
							>{description.length}/{MAX_DESCRIPTION}</span
						>
					</div>

					<div class="flex-1">
						<textarea
							name="description"
							bind:value={description}
							class="min-h-52 w-full resize-none border-0 bg-transparent px-6 py-5 text-base leading-7 text-base-content placeholder:text-base-content/40 shadow-none outline-none focus:outline-none focus:ring-0 sm:px-8"
							placeholder="Write a caption..."
							maxlength={MAX_DESCRIPTION}
							autocomplete="off"></textarea>
					</div>

					{#if errorMessage}
						<div
							class="mx-6 mb-6 rounded-2xl bg-error/10 px-4 py-3 text-sm font-bold text-error sm:mx-8"
						>
							{errorMessage}
						</div>
					{/if}

					<div
						class="border-t border-base-300 px-6 py-4 text-sm leading-6 text-base-content/60 sm:px-8"
					>
						Your post will publish to the feed after the image upload completes.
					</div>
				</div>
			</div>
		</form>
	</div>
</div>
