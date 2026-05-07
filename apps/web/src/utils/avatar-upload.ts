/**
 * Upload an image file to the avatar endpoint and return the publicly
 * accessible URL that can be stored as avatar_url.
 */
export async function uploadAvatarImage(file: File): Promise<string> {
  const apiBase = (import.meta.env.VITE_API_URL as string | undefined)?.trim() || '/api'
  const token = localStorage.getItem('token')

  const formData = new FormData()
  formData.append('image', file)

  const response = await fetch(`${apiBase}/avatars/upload`, {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    body: formData,
  })

  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as { message?: string }
    throw new Error(body.message ?? `Upload failed (${response.status})`)
  }

  const data = await response.json() as { url: string }
  // data.url is like "/avatars/{hash}.ext" — prepend API base so the
  // browser resolves it through the same proxy that serves the API.
  const avatarPath = data.url.startsWith('/') ? data.url : `/${data.url}`
  return `${apiBase}${avatarPath}`
}
