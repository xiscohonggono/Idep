export interface UserProfile {
    id: string;
    email: string;
    name: string;
}

export interface UserService {
    getUser(id: string): Promise<UserProfile>;
}