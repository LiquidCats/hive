<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { ref } from 'vue'
import { KeyRound, LoaderCircle } from '@lucide/vue'
import { useRouter } from 'vue-router'

import { RouteNames } from '@/app/router'
import { cn } from '@/shared/lib/utils'
import { Button } from '@/shared/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { useAuthStore } from '@/app/stores/auth'
import { usePasskey } from '@/shared/composables/usePasskey'

const props = defineProps<{
  class?: HTMLAttributes['class']
}>()

const router = useRouter()
const auth = useAuthStore()
const name = ref('')
const awaitingApproval = ref(false)
const { pending, error, signUp } = usePasskey()

async function submit() {
  const status = await signUp(name.value.trim())
  if (status === 'pending_approval') awaitingApproval.value = true
  if (status !== 'active') return
  // Only the server can confirm the session cookie took.
  await auth.load(true)
  if (auth.user) router.push({ name: RouteNames.HiveApps })
  else error.value = 'Account created, but no session was started. Try signing in.'
}
</script>

<template>
  <div :class="cn('flex flex-col gap-6', props.class)">
    <Card>
      <CardHeader class="text-center">
        <CardTitle class="text-xl">Create your account</CardTitle>
        <CardDescription>Register with a passkey</CardDescription>
      </CardHeader>
      <CardContent>
        <p v-if="awaitingApproval" class="text-center text-sm" role="status">
          Account created. Another user must approve it before you can sign in.
        </p>
        <form v-else @submit.prevent="submit">
          <FieldGroup>
            <Field>
              <FieldLabel for="username">Username</FieldLabel>
              <Input
                id="username"
                v-model="name"
                required
                autofocus
                minlength="3"
                maxlength="64"
                pattern="[a-zA-Z0-9][a-zA-Z0-9._\-]{2,63}"
                autocomplete="username"
                placeholder="admin"
              />
              <FieldDescription class="text-sm"
                >3–64 letters, digits, ".", "_" or "-", starting with a letter or digit. Your
                passkey is saved under this name.</FieldDescription
              >
            </Field>

            <Field>
              <Button type="submit" size="lg" :disabled="pending || !name.trim()">
                <LoaderCircle v-if="pending" class="animate-spin" />
                <KeyRound v-else />
                {{ pending ? 'Waiting for passkey…' : 'Sign up with passkey' }}
              </Button>
              <FieldError class="text-center" :errors="[error ?? undefined]" />
            </Field>
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  </div>
</template>
