import Layout from "./components/Layout";
import CreatePage from "./pages/CreatePage";
import NotePage from "./pages/NotePage";

export default function App() {
  // The Go server returns this same page for "/" and for "/n/<id>".
  // The app looks at the address to decide what to show.
  const match = window.location.pathname.match(/^\/n\/([^/]+)\/?$/);

  return (
    <Layout>{match ? <NotePage id={match[1]} /> : <CreatePage />}</Layout>
  );
}
