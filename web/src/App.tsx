import { BrowserRouter, Route, Routes } from "react-router-dom";
import Shell from "./Shell";
import { AppStateProvider } from "./store";
import Applications from "./pages/Applications";
import Containers from "./pages/Containers";
import DiscoveryRules from "./pages/DiscoveryRules";
import Inspector from "./pages/Inspector";
import Settings from "./pages/Settings";
import System from "./pages/System";

export default function App() {
  return (
    <AppStateProvider>
      <BrowserRouter>
        <Routes>
          <Route element={<Shell />}>
            <Route index element={<Applications />} />
            <Route path="containers" element={<Containers />} />
            <Route path="containers/:id/discovery" element={<Inspector />} />
            <Route path="system" element={<System />} />
            <Route path="rules" element={<DiscoveryRules />} />
            <Route path="settings" element={<Settings />} />
            <Route path="*" element={<Applications />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </AppStateProvider>
  );
}
