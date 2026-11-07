import type { paths } from '@/lib/api/schema';

export type User =
  paths['/api/v1/users/{id}']['get']['responses'][200]['content']['application/json'];

export async function getUser(id: string): Promise<User> {
  const baseUrl = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';
  const res = await fetch(`${baseUrl}/api/v1/users/${encodeURIComponent(id)}`, {
    cache: 'no-store',
  });

  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`);
  }

  return res.json();
}
