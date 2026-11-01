import { afterEach, describe, expect, it, vi } from 'vitest';
import { getUser } from '../lib/api/go';

const user = { id: '101', email: 'a@example.com', name: 'Ada' };

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

    await expect(getUser('101')).resolves.toEqual(user);
  });

  it.each([404, 503])('throws on %i', async (status) => {
    stubFetch({ title: 'error' }, status);

    await expect(getUser('101')).rejects.toThrow(String(status));
  });
});
