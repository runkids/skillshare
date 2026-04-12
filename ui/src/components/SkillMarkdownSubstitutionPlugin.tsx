import { useEffect } from 'react';
import { registerLexicalTextEntity } from '@lexical/text';
import { useCellValue } from '@mdxeditor/gurx';
import {
  addComposerChild$,
  addLexicalNode$,
  realmPlugin,
  rootEditor$,
} from '@mdxeditor/editor';
import {
  $applyNodeReplacement,
  TextNode,
  type EditorConfig,
  type LexicalUpdateJSON,
  type NodeKey,
  type SerializedTextNode,
  type Spread,
} from 'lexical';
import { getFirstSubstitutionTokenMatch } from '../lib/substitutionTokens';

type SerializedSubstitutionTokenNode = Spread<{
  type: 'substitution-token';
  version: 1;
}, SerializedTextNode>;

class SubstitutionTokenNode extends TextNode {
  static getType() {
    return 'substitution-token';
  }

  static clone(node: SubstitutionTokenNode) {
    return new SubstitutionTokenNode(node.__text, node.__key);
  }

  static importJSON(serializedNode: SerializedSubstitutionTokenNode) {
    return $createSubstitutionTokenNode(serializedNode.text).updateFromJSON(serializedNode);
  }

  constructor(text = '', key?: NodeKey) {
    super(text, key);
  }

  createDOM(config: EditorConfig): HTMLElement {
    const element = super.createDOM(config);
    element.classList.add('ss-substitution-token', 'ss-editor-substitution-token');
    return element;
  }

  updateDOM(prevNode: this, dom: HTMLElement, config: EditorConfig): boolean {
    const didUpdate = super.updateDOM(prevNode, dom, config);
    dom.classList.add('ss-substitution-token', 'ss-editor-substitution-token');
    return didUpdate;
  }

  updateFromJSON(serializedNode: LexicalUpdateJSON<SerializedSubstitutionTokenNode>): this {
    return super.updateFromJSON(serializedNode);
  }

  exportJSON(): SerializedSubstitutionTokenNode {
    return {
      ...super.exportJSON(),
      type: 'substitution-token',
      version: 1,
    };
  }

  canInsertTextBefore(): false {
    return false;
  }

  canInsertTextAfter(): false {
    return false;
  }

  isTextEntity(): true {
    return true;
  }
}

function $createSubstitutionTokenNode(text = '') {
  return $applyNodeReplacement(new SubstitutionTokenNode(text));
}

function SubstitutionTokenDecoratorPlugin() {
  const editor = useCellValue(rootEditor$);

  useEffect(() => {
    if (!editor) return;

    const unregister = registerLexicalTextEntity(
      editor,
      (text) => {
        const match = getFirstSubstitutionTokenMatch(text);
        if (!match) return null;
        return {
          start: match.start,
          end: match.end,
        };
      },
      SubstitutionTokenNode,
      (textNode) => $createSubstitutionTokenNode(textNode.getTextContent()),
    );

    return () => {
      unregister.forEach((cleanup) => cleanup());
    };
  }, [editor]);

  return null;
}

export const skillMarkdownSubstitutionPlugin = realmPlugin({
  init(realm) {
    realm.pub(addLexicalNode$, SubstitutionTokenNode);
    realm.pub(addComposerChild$, SubstitutionTokenDecoratorPlugin);
  },
});
