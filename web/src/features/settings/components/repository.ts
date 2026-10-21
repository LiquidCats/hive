export interface Repository {
  id: string
  url: string
  name: string
  username: string
  insecure: boolean
  hasPassword: boolean
  hasSshPrivateKey: boolean
}

// Same rule as the backend: ssh:// or scp-like user@host:path.
export const isSSH = (url: string) => url.startsWith('ssh://') || /^[\w.-]+@[\w.-]+:/.test(url)
