import { createRouter, createWebHistory } from "vue-router"

import ChatView from "../views/ChatView.vue"
import CreatePostView from "../views/CreatePostView.vue"
import FeedView from "../views/FeedView.vue"
import ForgotPasswordView from "../views/ForgotPasswordView.vue"
import GroupsView from "../views/GroupsView.vue"
import LoginView from "../views/LoginView.vue"
import MyPostsView from "../views/MyPostsView.vue"
import NotificationsView from "../views/NotificationsView.vue"
import ProfileView from "../views/ProfileView.vue"
import RegisterView from "../views/RegisterView.vue"
import ResetPasswordView from "../views/ResetPasswordView.vue"

const routes = [
  { path: "/", redirect: "/feed" },
  { path: "/feed", name: "feed", component: FeedView },
  { path: "/create", name: "create-post", component: CreatePostView },
  { path: "/profile/:handle?", name: "profile", component: ProfileView, props: true },
  { path: "/my-posts", name: "my-posts", component: MyPostsView },
  { path: "/groups", name: "groups", component: GroupsView },
  { path: "/notifications", name: "notifications", component: NotificationsView },
  { path: "/chat", name: "chat", component: ChatView },
  { path: "/login", name: "login", component: LoginView },
  { path: "/register", name: "register", component: RegisterView },
  { path: "/forgot-password", name: "forgot-password", component: ForgotPasswordView },
  { path: "/reset-password", name: "reset-password", component: ResetPasswordView }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

export default router
