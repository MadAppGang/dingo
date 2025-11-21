import React, { useState, useEffect } from 'react';
import { auth, db, signInWithGoogle, signInWithGitHub } from '../../lib/firebase';
import { onAuthStateChanged, type User } from 'firebase/auth';
import {
    collection,
    addDoc,
    query,
    where,
    orderBy,
    onSnapshot,
    serverTimestamp,
    type Timestamp
} from 'firebase/firestore';

interface Comment {
    id: string;
    text: string;
    userId: string;
    userName: string;
    userPhoto: string;
    createdAt: Timestamp;
    slug: string;
}

interface CommentSectionProps {
    slug: string;
}

export default function CommentSection({ slug }: CommentSectionProps) {
    const [user, setUser] = useState<User | null>(null);
    const [comments, setComments] = useState<Comment[]>([]);
    const [newComment, setNewComment] = useState('');
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    // Auth listener
    useEffect(() => {
        if (typeof window === 'undefined') return;

        try {
            const unsubscribe = onAuthStateChanged(auth, (currentUser) => {
                setUser(currentUser);
            });
            return () => unsubscribe();
        } catch (err) {
            console.error("Auth initialization error:", err);
            setError("Authentication failed to initialize");
        }
    }, []);

    // Comments listener - loads for everyone (not just signed-in users)
    useEffect(() => {
        if (!slug || typeof window === 'undefined' || !db) {
            setLoading(false);
            return;
        }

        try {
            const q = query(
                collection(db, 'comments'),
                where('slug', '==', slug),
                orderBy('createdAt', 'desc')
            );

            const unsubscribe = onSnapshot(
                q,
                (snapshot) => {
                    const commentsData = snapshot.docs.map(doc => ({
                        id: doc.id,
                        ...doc.data()
                    })) as Comment[];
                    setComments(commentsData);
                    setLoading(false);
                },
                (error) => {
                    console.error('Error loading comments:', error);
                    setError('Failed to load comments');
                    setLoading(false);
                }
            );

            return () => unsubscribe();
        } catch (err) {
            console.error('Error setting up comments listener:', err);
            setLoading(false);
        }
    }, [slug]);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!user || !newComment.trim()) return;

        try {
            await addDoc(collection(db, 'comments'), {
                text: newComment.trim(),
                userId: user.uid,
                userName: user.displayName || 'Anonymous',
                userPhoto: user.photoURL || '',
                createdAt: serverTimestamp(),
                slug: slug
            });
            setNewComment('');
        } catch (error) {
            console.error('Error adding comment:', error);
            alert('Failed to post comment. Please try again.');
        }
    };

    const formatDate = (timestamp: Timestamp) => {
        if (!timestamp) return '';
        return new Date(timestamp.seconds * 1000).toLocaleDateString(undefined, {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        });
    };

    if (typeof slug !== 'string') {
        console.error('CommentSection: Invalid slug prop:', slug);
        return null;
    }

    return (
        <div className="mt-12 border-t border-gray-200 pt-8">
            <h2 className="text-2xl font-bold mb-6 text-gray-900">Discussion</h2>

            {/* Comment Form */}
            <div className="mb-8">
                {user ? (
                    <form onSubmit={handleSubmit} className="space-y-4">
                        <div className="flex items-start gap-4">
                            <img
                                src={user.photoURL || `https://ui-avatars.com/api/?name=${user.displayName}`}
                                alt={user.displayName || 'User'}
                                className="w-10 h-10 rounded-full"
                            />
                            <div className="flex-1">
                                <textarea
                                    value={newComment}
                                    onChange={(e) => setNewComment(e.target.value)}
                                    placeholder="Join the conversation..."
                                    className="w-full bg-gray-900 border border-gray-700 rounded-lg p-3 text-white focus:ring-2 focus:ring-orange-500 focus:border-transparent min-h-[100px]"
                                />
                                <div className="mt-2 flex justify-end">
                                    <button
                                        type="submit"
                                        disabled={!newComment.trim()}
                                        className="px-4 py-2 bg-orange-600 text-white rounded-lg hover:bg-orange-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                                    >
                                        Post Comment
                                    </button>
                                </div>
                            </div>
                        </div>
                    </form>
                ) : (
                    <div className="bg-white border border-gray-200 rounded-2xl p-8 shadow-lg max-w-md mx-auto">
                        <h3 className="text-2xl font-bold text-gray-900 mb-3">Sign in to continue</h3>
                        <p className="text-gray-600 mb-6 leading-relaxed">
                            Join the discussion to share your thoughts, ask questions, and connect with the Dingo community.
                        </p>
                        <div className="h-px bg-gray-200 mb-6"></div>
                        <div className="space-y-3">
                            <button
                                onClick={() => signInWithGitHub()}
                                className="w-full px-6 py-3.5 bg-[#24292e] text-white rounded-xl hover:bg-[#2f363d] font-medium transition-all duration-200 flex items-center justify-center gap-3 shadow-sm hover:shadow-md"
                            >
                                <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                                    <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
                                </svg>
                                Continue with GitHub
                            </button>
                            <button
                                onClick={() => signInWithGoogle()}
                                className="w-full px-6 py-3.5 bg-white text-gray-900 rounded-xl hover:bg-gray-50 font-medium transition-all duration-200 flex items-center justify-center gap-3 border-2 border-gray-200 hover:border-gray-300 shadow-sm hover:shadow-md"
                            >
                                <svg className="w-5 h-5" viewBox="0 0 24 24">
                                    <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4" />
                                    <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853" />
                                    <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05" />
                                    <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335" />
                                </svg>
                                Continue with Google
                            </button>
                        </div>
                    </div>
                )}
            </div>

            {/* Comments List */}
            <div className="space-y-6">
                {error ? (
                    <div className="text-center py-8 px-6 bg-red-50 border border-red-200 rounded-xl">
                        <p className="text-red-600 font-medium">{error}</p>
                        <p className="text-red-500 text-sm mt-1">Please refresh the page to try again</p>
                    </div>
                ) : loading ? (
                    <div className="text-center py-12">
                        <div className="inline-block w-8 h-8 border-3 border-gray-300 border-t-orange-500 rounded-full animate-spin"></div>
                        <p className="text-gray-500 mt-3">Loading comments...</p>
                    </div>
                ) : comments.length === 0 ? (
                    <div className="text-center py-16 px-6">
                        <div className="inline-flex items-center justify-center w-16 h-16 bg-gray-100 rounded-full mb-4">
                            <svg className="w-8 h-8 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
                            </svg>
                        </div>
                        <h3 className="text-lg font-semibold text-gray-900 mb-2">No comments yet</h3>
                        <p className="text-gray-500">Be the first to share your thoughts!</p>
                    </div>
                ) : (
                    comments.map((comment) => (
                        <div key={comment.id} className="flex gap-4 group">
                            <img
                                src={comment.userPhoto || `https://ui-avatars.com/api/?name=${comment.userName}`}
                                alt={comment.userName}
                                className="w-10 h-10 rounded-full"
                            />
                            <div className="flex-1">
                                <div className="flex items-baseline gap-2 mb-1">
                                    <span className="font-medium text-gray-900">{comment.userName}</span>
                                    <span className="text-xs text-gray-500">{formatDate(comment.createdAt)}</span>
                                </div>
                                <p className="text-gray-700 leading-relaxed">{comment.text}</p>
                            </div>
                        </div>
                    ))
                )}
            </div>
        </div>
    );
}
