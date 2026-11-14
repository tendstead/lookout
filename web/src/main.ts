import { mount } from 'svelte';
import App from './App.svelte';
import '@tendstead/ui/theme.css';
import './style.css';
mount(App, { target: document.getElementById('app')! });
