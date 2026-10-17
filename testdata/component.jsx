export function List({ items }) {
  return items.map((i) => <li key={i}>{i}</li>);
}

export default class App extends React.Component {
  render() {
    return <List items={[1, 2]} />;
  }
}
