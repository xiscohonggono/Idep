import { UserService, UserProfile } from './types';

const GO_API_URL = process.env.NEXT_PUBLIC_GO_API_URL || 'http://localhost:8080';

export const goUserService: UserService = {
    async getUser(id: string): Promise<UserProfile> {
        const res = await fetch(`${GO_API_URL}/api/v1/users/${id}`, {
            cache: 'no-store',
        });

        if (!res.ok) {
            throw new Error(`Go API error: ${res.statusText}`);
        }

        return res.json();
    },
};
