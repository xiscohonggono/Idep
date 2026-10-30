import { UserService } from './types';
import { supabaseUserService } from './supabase';
import { goUserService } from './go';

// Read backend provider from environment variable; defaults to "supabase"
const backendProvider = process.env.NEXT_PUBLIC_BACKEND_PROVIDER || 'supabase';

export const userService: UserService =
    backendProvider === 'go' ? goUserService : supabaseUserService;