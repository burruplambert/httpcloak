import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    'index',
    {
      type: 'category',
      label: 'Concepts',
      link: {type: 'doc', id: 'concepts/index'},
      items: [],
    },
    {
      type: 'category',
      label: 'Reference',
      link: {type: 'doc', id: 'reference/index'},
      items: [],
    },
    {
      type: 'category',
      label: 'How-to',
      link: {type: 'doc', id: 'how-to/index'},
      items: [],
    },
    {
      type: 'category',
      label: 'Tutorial',
      link: {type: 'doc', id: 'tutorial/index'},
      items: [],
    },
  ],
};

export default sidebars;
