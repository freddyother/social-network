<template>
  <section class="page">
    <div class="panel">
      <p class="eyebrow">Feed</p>
      <h2>Posts, comments, and privacy</h2>
      <p>
        This view is ready to consume the main feed, create posts with an image or GIF, and apply public, followers-only, or custom visibility.
      </p>
    </div>

    <div class="grid grid--two">
      <article class="panel">
        <h3>Expected API</h3>
        <p>`GET /posts`, `POST /posts`, `POST /posts/:id/comments`</p>
      </article>
      <article class="panel">
        <h3>UI state</h3>
        <p>composer, privacy filters, infinite list, and optimistic feedback.</p>
      </article>
    </div>
  </section>
</template>
