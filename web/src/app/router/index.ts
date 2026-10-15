import AuthScreen from '@/features/auth/AuthScreen.vue'
import SignIn from '@/features/auth/components/SignIn.vue'
import SignUp from '@/features/auth/components/SignUp.vue'
import BaseLayout from '@/shared/components/BaseLayout.vue'
import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/app/stores/auth'
import SecretsScreen from '@/features/secrets/SecretsScreen.vue'
import ApplicationsScreen from '@/features/applications/ApplicationsScreen.vue'
import SettingsScreen from '@/features/settings/SettingsScreen.vue'
import NodesScreen from '@/features/nodes/NodesScreen.vue'
import UserList from '@/features/users/UserList.vue'

export enum RouteNames {
  Hive = 'home',
  HiveApps = 'hive.appications',
  HiveSecrets = 'hive.secrets',
  HiveNodes = 'hive.nodes',
  HiveUsers = 'hive.users',
  HiveSettings = 'hive.settings',

  AuthSignIn = 'auth.sign-in',
  AuthSignUp = 'auth.sign-up',
}

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: RouteNames.Hive,
      component: BaseLayout,
      redirect: { name: RouteNames.HiveApps },
      meta: {
        requiresAuth: true,
      },
      children: [
        {
          path: 'apps',
          name: RouteNames.HiveApps,
          component: ApplicationsScreen,
          meta: {
            header: 'Applications',
          },
        },
        {
          path: 'nodes',
          name: RouteNames.HiveNodes,
          component: NodesScreen,
          meta: {
            header: 'Nodes',
          },
        },
        {
          path: 'secrets',
          name: RouteNames.HiveSecrets,
          component: SecretsScreen,
          meta: {
            header: 'Secrets',
          },
        },
        {
          path: 'users',
          name: RouteNames.HiveUsers,
          component: UserList,
          meta: {
            header: 'Users',
          },
        },
        {
          path: 'settings',
          name: RouteNames.HiveSettings,
          component: SettingsScreen,
          meta: {
            header: 'Settings',
          },
        },
      ],
    },
    {
      path: '/auth',
      component: AuthScreen,
      meta: { guestOnly: true },
      children: [
        {
          path: 'sign-in',
          name: RouteNames.AuthSignIn,
          component: SignIn,
        },
        {
          path: 'sign-up',
          name: RouteNames.AuthSignUp,
          component: SignUp,
        },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  await auth.load()
  if (to.meta.requiresAuth && !auth.user) return { name: RouteNames.AuthSignIn }
  if (to.meta.guestOnly && auth.user) return { name: RouteNames.HiveApps }
})

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    guestOnly?: boolean
  }
}

export default router
