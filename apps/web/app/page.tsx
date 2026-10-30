import { userService } from '../lib/api';

export default async function HomePage() {
  const user = await userService.getUser('101');
  const provider = process.env.NEXT_PUBLIC_BACKEND_PROVIDER || 'supabase';

  return (
    <main style={{ padding: '2rem', fontFamily: 'sans-serif' }}>
      <h1>Architecture Switch Demo</h1>
      <p>
        <strong>Current Backend Mode:</strong>{' '}
        <span style={{ color: provider === 'go' ? 'green' : 'blue' }}>
          {provider.toUpperCase()}
        </span>
      </p>
      <h3>User Data Received:</h3>
      <pre style={{ background: '#f4f4f4', padding: '1rem', borderRadius: '6px' }}>
        {JSON.stringify(user, null, 2)}
      </pre>
    </main>
  );
}