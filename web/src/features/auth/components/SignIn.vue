<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { KeyRound, LoaderCircle } from '@lucide/vue'
import { useRouter } from 'vue-router'

import { RouteNames } from '@/app/router'
import { cn } from '@/shared/lib/utils'
import { Button } from '@/shared/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { Field, FieldError, FieldGroup } from '@/shared/ui/field'
import { useAuthStore } from '@/app/stores/auth'
import { usePasskey } from '@/shared/composables/usePasskey'

const props = defineProps<{
  class?: HTMLAttributes['class']
}>()

const router = useRouter()
const auth = useAuthStore()
const { pending, error, signIn } = usePasskey()

async function submit() {
  if (!(await signIn())) return
  // Only the server can confirm the session cookie took.
  await auth.load(true)
  if (auth.user) router.push({ name: RouteNames.HiveApps })
  else error.value = 'Signed in, but no session was started. Try again.'
}
</script>

<template>
  <div :class="cn('flex flex-col gap-6', props.class)">
    <Card>
      <CardHeader class="text-center">
        <CardTitle class="text-xl">Welcome back</CardTitle>
        <CardDescription>Sign in with your passkey</CardDescription>
      </CardHeader>
      <CardContent>
        <form @submit.prevent="submit">
          <FieldGroup>
            <Field>
              <Button type="submit" size="lg" :disabled="pending">
                <LoaderCircle v-if="pending" class="animate-spin" />
                <KeyRound v-else />
                {{ pending ? 'Waiting for passkey…' : 'Sign in with passkey' }}
              </Button>
              <FieldError class="text-center" :errors="[error ?? undefined]" />
            </Field>
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  </div>
</template>
