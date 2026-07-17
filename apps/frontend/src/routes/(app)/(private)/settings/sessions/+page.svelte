<script lang="ts">
	import { resolve } from '$app/paths';
	import { enhance } from '$app/forms';
	import { ArrowLeft, Monitor, Trash2 } from '@lucide/svelte';
	import type { Session } from '$lib/types';
	import type { ActionData, PageData } from './$types';

	let { data, form }: { data: PageData; form: ActionData } = $props();

	let revoked = $state<Record<string, boolean>>({});
	let revoking = $state<Record<string, boolean>>({});
	let actionError = $state('');
	let sessions = $derived<Session[]>(data.sessions.filter((session) => !revoked[session.id]));

	const timestamp = new Intl.DateTimeFormat('en', {
		dateStyle: 'medium',
		timeStyle: 'short',
		timeZone: 'UTC'
	});

	function formatTimestamp(value: Date): string {
		return `${timestamp.format(value)} UTC`;
	}
</script>

<svelte:head>
	<title>Active Sessions — Phasma</title>
</svelte:head>

<div
	class="mx-auto flex max-w-xl flex-col gap-6 rounded-2xl border border-base-300 bg-base-100 p-6 text-base-content shadow-lg shadow-slate-900/5 sm:px-8"
>
	<div class="flex items-start gap-3 border-b border-base-300 pb-4">
		<a
			href={resolve('/settings')}
			class="btn btn-ghost btn-circle btn-sm shrink-0 text-base-content transition-colors hover:bg-base-200"
			title="Back to Settings"
			aria-label="Back to Settings"
		>
			<ArrowLeft class="h-5 w-5" />
		</a>
		<div class="grid gap-1">
			<h1 class="text-2xl font-black text-base-content">Active Sessions</h1>
			<p class="text-xs leading-5 text-base-content/65">
				Review browsers signed in to your account and revoke sessions you no longer use.
			</p>
		</div>
	</div>

	{#if data.loadError}
		<div
			class="rounded-2xl border border-error/30 bg-error/10 px-4 py-3 text-sm font-semibold text-error"
			role="alert"
		>
			{data.loadError}
		</div>
	{:else}
		{#if actionError || form?.error}
			<div
				class="rounded-2xl border border-error/30 bg-error/10 px-4 py-3 text-sm font-semibold text-error"
				role="alert"
			>
				{actionError || form?.error}
			</div>
		{/if}

		{#if sessions.length === 0}
			<div
				class="rounded-2xl border border-dashed border-base-300 bg-base-200 px-5 py-10 text-center"
			>
				<Monitor class="mx-auto mb-3 h-8 w-8 text-base-content" />
				<p class="text-sm font-bold">No active sessions found.</p>
			</div>
		{:else}
			<ul class="grid gap-3" aria-label="Active browser sessions">
				{#each sessions as session (session.id)}
					<li class="relative rounded-2xl border border-base-300 bg-base-200 p-4">
						<div class="flex items-start gap-3 pr-12">
							<div class="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-base-100">
								<Monitor class="h-5 w-5 text-base-content" />
							</div>
							<div class="min-w-0 flex-1">
								<div class="flex flex-wrap items-center gap-2">
									<h2 class="text-sm font-black">Browser session</h2>
									{#if session.current}
										<span class="badge badge-neutral badge-sm font-bold">Current</span>
									{/if}
								</div>
								<dl class="mt-3 grid gap-2 text-xs text-base-content/65 sm:grid-cols-2">
									<div>
										<dt class="font-bold text-base-content/80">Created</dt>
										<dd>
											<time datetime={session.created.toISOString()}
												>{formatTimestamp(session.created)}</time
											>
										</dd>
									</div>
									<div>
										<dt class="font-bold text-base-content/80">Expires</dt>
										<dd>
											<time datetime={session.expiresAt.toISOString()}
												>{formatTimestamp(session.expiresAt)}</time
											>
										</dd>
									</div>
								</dl>
							</div>
						</div>
						{#if !session.current}
							<form
								method="POST"
								action="?/revoke"
								class="absolute right-3 top-3"
								use:enhance={() => {
									revoking[session.id] = true;
									actionError = '';
									return async ({ result, update }) => {
										delete revoking[session.id];
										if (result.type === 'success') {
											revoked[session.id] = true;
										} else if (result.type === 'failure') {
											actionError =
												(result.data as { error?: string })?.error ??
												'Could not revoke the session. Please try again.';
										} else if (result.type === 'error') {
											actionError = 'Could not revoke the session. Please try again.';
										}
										await update({ reset: false, invalidateAll: false });
									};
								}}
							>
								<input type="hidden" name="sessionId" value={session.id} />
								<button
									type="submit"
									disabled={revoking[session.id]}
									class="inline-flex h-10 w-10 items-center justify-center rounded-full text-base-content transition-all duration-150 hover:bg-error/10 hover:text-error active:scale-95 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-error/50 disabled:pointer-events-none disabled:opacity-50"
									title="Revoke session"
									aria-label="Revoke session"
								>
									{#if revoking[session.id]}
										<span class="loading loading-spinner loading-sm"></span>
									{:else}
										<Trash2 class="h-5 w-5" />
									{/if}
								</button>
							</form>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	{/if}
</div>
