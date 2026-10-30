import { UserService, UserProfile } from './types';

export const supabaseUserService: UserService = {
    async getUser(id: string): Promise<UserProfile> {
        // In Hackathon mode: query Supabase client directly
        // e.g., const { data } = await supabase.from('users').select('*').eq('id', id).single();
        return {
            id,
            email: `hackathon_${id}@supabase.io`,
            name: "Supabase Fast Track User",
        };
    },
};