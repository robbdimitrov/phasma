<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { pageTitle } from '$lib/pageTitle';
</script>

<svelte:head>
	<title>{pageTitle(String(page.status))}</title>
</svelte:head>

<div class="flex min-h-screen items-center justify-center bg-base-200 p-4">
	<div
		class="flex w-full max-w-sm flex-col items-center gap-6 rounded-2xl border border-base-300 bg-base-100 p-8 text-center text-base-content shadow-xl shadow-slate-900/5"
	>
		<div class="grid h-16 w-16 place-items-center rounded-full bg-neutral text-neutral-content">
			<span class="text-2xl font-black">{page.status}</span>
		</div>
		<div class="grid gap-2">
			<h1 class="text-2xl font-black">
				{#if page.status === 404}
					Page not found
				{:else if page.status === 403}
					Access denied
				{:else}
					Something went wrong
				{/if}
			</h1>
			<p class="text-sm leading-6 text-base-content/70">
				{page.error?.message ?? 'An unexpected error occurred.'}
			</p>
		</div>
		<a href={resolve('/feed')} class="btn btn-neutral h-11 min-h-11 rounded-full px-6 font-bold">
			Back to Feed
		</a>
	</div>
</div>
