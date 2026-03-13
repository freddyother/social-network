<template>
  <section class="page">
    <div class="panel">
      <p class="eyebrow">Auth</p>
      <h2>Register</h2>
      <form class="form-grid">
        <label>
          <span>First Name</span>
          <input type="text" placeholder="Ada" />
        </label>
        <label>
          <span>Last Name</span>
          <input type="text" placeholder="Lovelace" />
        </label>
        <label>
          <span>Email</span>
          <input type="email" placeholder="ada@example.com" />
        </label>
        <label>
          <span>Password</span>
          <input type="password" placeholder="********" />
        </label>
        <label>
          <span>Date of Birth</span>
          <input type="date" />
        </label>
        <label>
          <span>Nickname</span>
          <input type="text" placeholder="Optional" />
        </label>
        <label class="form-grid__full">
          <span>About Me</span>
          <textarea rows="4" placeholder="Tell your story in a few lines"></textarea>
        </label>
        <button type="button" class="button form-grid__full">
          Create account
        </button>
      </form>
    </div>
  </section>
</template>
