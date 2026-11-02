import { getUser } from '@/lib/api';

export default async function HomePage() {
  let result: string;
  try {
    result = JSON.stringify(await getUser('00000000-0000-0000-0000-000000000101'), null, 2);
  } catch (err) {
    result = err instanceof Error ? err.message : String(err);
  }

  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-4 font-sans">
      <h1 className="text-2xl font-bold">Go API Demo</h1>
      <pre className="rounded-md bg-gray-100 text-sm text-gray-900">{result}</pre>
    </main>
  );
}
