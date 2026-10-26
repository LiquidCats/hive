import { ref } from 'vue'

// Native WebAuthn JSON helpers (Chrome 129+, Safari 18.4+, Firefox 119+): no base64url lib needed.
// Absolute API origin from VITE_API_URL (e.g. https://api.example.com); empty = same origin.
const API_URL = (import.meta.env.VITE_API_URL ?? '').replace(/\/+$/, '')

export class ApiError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.status = status
  }
}

// The one BFF fetch helper: session cookie included, JSON in and out, server error message surfaced.
export async function fetchJSON<T>(
  path: string,
  init: { method?: string; body?: unknown } = {},
): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    method: init.method ?? 'GET',
    credentials: 'include', // session cookie on cross-origin API; needs CORS allow-credentials
    headers: init.body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: init.body === undefined ? undefined : JSON.stringify(init.body),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => null)
    throw new ApiError(err?.message ?? res.statusText, res.status)
  }
  return res.headers.get('content-type')?.includes('json') ? res.json() : (undefined as T)
}

const post = <T>(path: string, body?: unknown) =>
  fetchJSON<T>(`/webauthn/${path}`, { method: 'POST', body })

export type RegistrationStatus = 'active' | 'pending_approval'

export function usePasskey() {
  const pending = ref(false)
  const error = ref<string | null>(null)

  // Resolves to fn's result, or undefined (with error set) if it failed.
  async function run<T>(fn: () => Promise<T>): Promise<T | undefined> {
    pending.value = true
    error.value = null
    try {
      // Browsers expose WebAuthn only on https (or localhost); fail before the start call writes a ceremony row.
      if (!window.isSecureContext) throw new Error('Passkeys need HTTPS: open this page via https://')
      return await fn()
    } catch (e) {
      // user cancelling the browser prompt throws NotAllowedError
      error.value = e instanceof Error ? e.message : 'Passkey failed'
    } finally {
      pending.value = false
    }
  }

  // Only an active account gets the HttpOnly session cookie; later sign-ups wait for approval.
  const signUp = (username: string) =>
    run(async () => {
      const { publicKey } = await post<{ publicKey: PublicKeyCredentialCreationOptionsJSON }>(
        'register/start',
        { username },
      )
      const cred = (await navigator.credentials.create({
        publicKey: PublicKeyCredential.parseCreationOptionsFromJSON(publicKey),
      })) as PublicKeyCredential | null
      if (!cred) throw new Error('Passkey creation cancelled')
      const { status } = await post<{ status: RegistrationStatus }>(
        'register/finish',
        cred.toJSON(),
      )
      return status
    })

  // Usernameless: the browser lists the passkeys it holds for this site.
  const signIn = () =>
    run(async () => {
      const { publicKey } = await post<{ publicKey: PublicKeyCredentialRequestOptionsJSON }>(
        'login/start',
      )
      const cred = (await navigator.credentials.get({
        publicKey: PublicKeyCredential.parseRequestOptionsFromJSON(publicKey),
      })) as PublicKeyCredential | null
      if (!cred) throw new Error('Passkey sign-in cancelled')
      await post('login/finish', cred.toJSON())
      return true
    })

  return { pending, error, signUp, signIn }
}
