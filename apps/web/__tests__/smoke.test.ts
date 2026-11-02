import { afterEach, describe, expect, it, vi } from 'vitest';
import { getUser, type User } from '../lib/api/go';

const userId = '00000000-0000-0000-0000-000000000101';
const user: User = {
  id: userId,
  email: 'a@example.com',
  display_name: 'Ada',
  email_notifications_enabled: false,
  is_verified_educator: false,
  roles: ['user'],
  created_at: '2026-11-02T00:00:00Z',
  updated_at: '2026-11-02T00:00:00Z',
};

function stubFetch(body: unknown, status: number) {
  const fetchMock = vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), { status }),
  );
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

describe('getUser', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.unstubAllEnvs();
  });

  it('builds the URL from NEXT_PUBLIC_API_URL and encodes the id', async () => {
    vi.stubEnv('NEXT_PUBLIC_API_URL', 'https://api.test');
    const fetchMock = stubFetch(user, 200);

    await getUser('a/b');

    expect(fetchMock).toHaveBeenCalledWith(
      'https://api.test/api/v1/users/a%2Fb',
      { cache: 'no-store' },
    );
  });

  it('returns the body on 200', async () => {
    stubFetch(user, 200);

    await expect(getUser(userId)).resolves.toEqual(user);
  });

  it.each([404, 503])('throws on %i', async (status) => {
    stubFetch({ title: 'error' }, status);

    await expect(getUser(userId)).rejects.toThrow(String(status));
  });
});
