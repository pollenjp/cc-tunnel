interface WindowEnv {
  BACKEND_URL?: string;
}

interface Window {
  __ENV__?: WindowEnv;
}
