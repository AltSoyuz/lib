<script lang="ts">
	import { tick, type Snippet } from 'svelte';
	import TitleBar from './TitleBar.svelte';
	import { errorText, request } from './api';
	import { text } from './text';

	// The sign-in by a code sent by email, against lib's backoffice.RequestCode (POST /api/v1/sign-in/code) and VerifyCode
	// (POST /api/v1/sign-in/verify), under the kit's title bar and its offline and new version banners. children: what goes
	// under the email step (another way in, an install link).
	let {
		title,
		logo,
		storageKey,
		onsignedin,
		children
	}: { title: string; logo: string; storageKey: string; onsignedin: () => void; children?: Snippet } = $props();

	const CODE_TTL = 15 * 60_000;
	const RESEND_AFTER = 30_000;

	interface Pending {
		email: string;
		sentAt: number;
	}

	// iOS often reloads a home screen app while the user reads the code in Mail: the code step outlives that.
	function readPending(): Pending | null {
		try {
			const p = JSON.parse(localStorage.getItem(storageKey) ?? 'null') as Pending | null;
			if (p && typeof p.email === 'string' && typeof p.sentAt === 'number' && Date.now() < p.sentAt + CODE_TTL) return p;
			localStorage.removeItem(storageKey);
		} catch (e) {
			console.warn('sign-in step not restored:', e);
		}
		return null;
	}

	function savePending(p: Pending | null) {
		try {
			if (p) localStorage.setItem(storageKey, JSON.stringify(p));
			else localStorage.removeItem(storageKey);
		} catch (e) {
			console.warn('sign-in step not saved:', e);
		}
	}

	const restored = readPending();
	let step = $state<'email' | 'code'>(restored ? 'code' : 'email');
	let email = $state(restored?.email ?? '');
	let sentAt = $state(restored?.sentAt ?? 0);
	let code = $state('');
	let sending = $state(false);
	let verifying = $state(false);
	let error = $state<string | null>(null);
	let codeError = $state<string | null>(null);
	let notice = $state<string | null>(null);
	let now = $state(Date.now());
	let emailInput = $state<HTMLInputElement>();
	let codeInput = $state<HTMLInputElement>();

	let resendIn = $derived(Math.max(0, Math.ceil((sentAt + RESEND_AFTER - now) / 1000)));

	$effect(() => {
		if (step !== 'code') return;
		const timer = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(timer);
	});

	$effect(() => {
		if (step === 'code') codeInput?.focus();
	});

	function messageOf(e: unknown): string {
		const message = (e as Error).message;
		if (message === 'too_many_codes') return text.tooManyCodes;
		if (message === 'rate_limit_exceeded') return text.rateLimited;
		if (message.startsWith('invalid_email')) return text.invalidEmail;
		if (message === 'invalid_or_expired_code') return text.wrongCode;
		if (message === 'too_many_attempts') return text.tooManyAttempts;
		if (message === 'user_restrained') return text.restrained;
		return errorText(e);
	}

	function showCodeStep(address: string) {
		email = address;
		sentAt = Date.now();
		now = sentAt;
		code = '';
		step = 'code';
		savePending({ email: address, sentAt });
	}

	async function sendCode(address: string) {
		if (!address || sending) return;
		const again = step === 'code';
		sending = true;
		error = null;
		codeError = null;
		notice = null;
		try {
			await request('POST', '/api/v1/sign-in/code', { email: address });
			showCodeStep(address);
			if (again) notice = text.newCodeSent;
		} catch (e) {
			// Over the limit no code is made, so the latest one emailed still works.
			if ((e as Error).message === 'too_many_codes') {
				showCodeStep(address);
				codeError = messageOf(e);
			} else if (again) {
				codeError = messageOf(e);
			} else {
				error = messageOf(e);
			}
		} finally {
			sending = false;
		}
	}

	async function verify() {
		if (verifying) return;
		if (code.length !== 6) {
			codeError = text.enterSixDigits;
			return;
		}
		verifying = true;
		codeError = null;
		notice = null;
		try {
			await request('POST', '/api/v1/sign-in/verify', { email, code });
			savePending(null);
			onsignedin();
		} catch (e) {
			codeError = messageOf(e);
			code = '';
		} finally {
			verifying = false;
		}
	}

	// Keeps the digits only, so a pasted "123 456" or "Code: 123456" works; the 6th digit submits.
	function setCode(input: HTMLInputElement, typed: string) {
		code = typed.replace(/\D/g, '').slice(0, 6);
		input.value = code;
		if (code) {
			codeError = null;
			notice = null;
		}
		if (code.length === 6) verify();
	}

	function pasteCode(e: ClipboardEvent & { currentTarget: EventTarget & HTMLInputElement }) {
		const pasted = e.clipboardData?.getData('text');
		if (!pasted) return;
		e.preventDefault();
		setCode(e.currentTarget, pasted);
	}

	async function useAnotherEmail() {
		savePending(null);
		step = 'email';
		code = '';
		error = null;
		codeError = null;
		notice = null;
		await tick();
		emailInput?.focus();
	}
</script>

<div class="sign-in">
	<img class="brand" src={logo} alt="" width="64" height="64" />
	<TitleBar {title} />

	{#if step === 'email'}
		<form
			onsubmit={(e) => {
				e.preventDefault();
				sendCode(email.trim());
			}}
		>
			<section class="section">
				<ul class="list">
					<li>
						<label class="cell field stacked">
							<span>{text.email}</span>
							<input
								type="email"
								name="email"
								bind:this={emailInput}
								bind:value={email}
								autocomplete="email"
								autocapitalize="off"
								spellcheck="false"
								required
								disabled={sending}
							/>
						</label>
					</li>
				</ul>
				{#if error}
					<p class="footer error" role="alert">{error}</p>
				{/if}
			</section>
			<button type="submit" class="filled wide" disabled={sending || !email.trim()}>{text.sendCode}</button>
		</form>
		{#if children}
			<div class="more">{@render children()}</div>
		{/if}
	{:else}
		<p class="lead">{text.codeSentTo} <strong>{email}</strong>.</p>
		<form
			onsubmit={(e) => {
				e.preventDefault();
				verify();
			}}
		>
			<section class="section">
				<ul class="list">
					<li>
						<label class="cell field stacked code">
							<span>{text.code}</span>
							<input
								type="text"
								name="code"
								bind:this={codeInput}
								value={code}
								oninput={(e) => setCode(e.currentTarget, e.currentTarget.value)}
								onpaste={pasteCode}
								autocomplete="one-time-code"
								inputmode="numeric"
								pattern="[0-9]*"
								maxlength="6"
								autocapitalize="off"
								spellcheck="false"
								readonly={verifying}
							/>
						</label>
					</li>
				</ul>
				{#if codeError}
					<p class="footer error" role="alert">{codeError}</p>
				{:else if notice}
					<p class="footer" role="status">{notice}</p>
				{/if}
			</section>
			<button type="submit" class="filled wide" disabled={verifying}>{verifying ? text.signingIn : text.signIn}</button>
		</form>

		<section class="section more">
			<ul class="list">
				<li>
					<button type="button" class="cell tint" onclick={() => sendCode(email)} disabled={sending || resendIn > 0}>
						{resendIn > 0 ? text.resendIn(resendIn) : text.resendCode}
					</button>
				</li>
				<li><button type="button" class="cell tint" onclick={useAnotherEmail}>{text.useAnotherEmail}</button></li>
			</ul>
		</section>
	{/if}
</div>

<style>
	.sign-in {
		max-width: 480px;
		margin: 0 auto;
	}

	.brand {
		display: block;
		margin: var(--space-5) 0 var(--space-4);
		border-radius: var(--radius-lg);
	}

	.lead {
		margin: 0 var(--space-4) var(--space-3);
		color: var(--muted-foreground);
		font-size: var(--text-subhead);
	}

	.lead strong {
		color: var(--foreground);
		font-weight: 600;
		overflow-wrap: anywhere;
	}

	.code input {
		font-size: var(--text-large-title);
		font-variant-numeric: tabular-nums;
		letter-spacing: 0.3em;
	}

	.more {
		margin-top: var(--space-6);
	}
</style>
