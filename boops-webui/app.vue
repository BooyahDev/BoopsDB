<template>
  <NuxtLayout>
    <header>
      <h1>BoopsDB!</h1>
      <nav>
        <NuxtLink to="/">Search</NuxtLink>
        <NuxtLink to="/machines/register">Register New Machine</NuxtLink>
      </nav>
    </header>
    <NuxtPage />
  </NuxtLayout>
</template>